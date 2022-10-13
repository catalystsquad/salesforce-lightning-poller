package pkg

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/catalystsquad/app-utils-go/errorutils"
	"github.com/catalystsquad/app-utils-go/logging"
	"github.com/catalystsquad/salesforce-utils/pkg"
	"github.com/dgraph-io/badger/v3"
	"github.com/go-playground/validator/v10"
	"github.com/joomcode/errorx"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type Position struct {
	LastModifiedDate  *time.Time
	NextURL           string
	PreviousRecordIDs []string
}

type LightningPoller struct {
	config    *RunConfig
	pollMap   *sync.Map
	db        *badger.DB
	SfUtils   *pkg.SalesforceUtils
	positions map[string]*Position
}

type RunConfig struct {
	Queries                  []QueryWithCallback `validate:"required"`
	StartupPositionOverrides map[string]time.Time
	Ticker                   *time.Ticker
	PersistenceEnabled       bool   `json:"persistence_enabled"`
	PersistencePath          string `json:"persistence_path"`
}

type QueryWithCallback struct {
	Query          func() string                       `json:"query" validate:"required"`
	PersistenceKey string                              `json:"persistenceKey"`
	Callback       func(result []byte, err error) bool `validate:"required"`
}

func NewLightningPoller(queries []QueryWithCallback, sfConfig pkg.Config) (*LightningPoller, error) {
	poller := &LightningPoller{
		pollMap: &sync.Map{},
	}
	config, err := initConfig(queries)
	if err != nil {
		return nil, err
	}
	poller.config = config
	poller.SfUtils, err = pkg.NewSalesforceUtils(true, sfConfig)
	if err != nil {
		return nil, err
	}
	return poller, err
}

func (p *LightningPoller) Run() {
	if p.config.PersistenceEnabled {
		err := p.openBadgerDb(p.config.PersistencePath)
		if err != nil {
			return
		}
	}
	defer p.closeBadgerDb()
	err := p.loadPositions()
	errorutils.PanicOnErr(nil, "error loading poller position", err)
	for range p.config.Ticker.C {
		p.poll()
	}
}

// loadPositions loads positions into memory, using saved state if saved state exists
func (p *LightningPoller) loadPositions() error {
	// init poller's positions map
	p.positions = map[string]*Position{}
	// load position for each query based on persistence key
	for _, query := range p.config.Queries {
		key := query.PersistenceKey
		// check if there is a position override for the persistence key
		if timeOverride, exists := p.config.StartupPositionOverrides[key]; exists {
			p.positions[key] = &Position{LastModifiedDate: &timeOverride}
		} else {
			if p.config.PersistenceEnabled {
				// fetch saved position and set it on the map
				savedPosition, err := p.getPosition([]byte(key))
				if err != nil {
					return err
				}
				p.positions[key] = savedPosition
			} else {
				// persistence is disabled, initialize to zero values
				p.positions[key] = &Position{LastModifiedDate: &time.Time{}}
			}
		}
	}
	return nil
}

func (p *LightningPoller) poll() {
	for _, queryWithCallback := range p.config.Queries {
		go func(queryWithCallback QueryWithCallback) {
			defer p.pollMap.Store(queryWithCallback.PersistenceKey, false)
			polling, ok := p.pollMap.Load(queryWithCallback.PersistenceKey)
			if !ok {
				// first poll, so set polling to true
				p.pollMap.Store(queryWithCallback.PersistenceKey, true)
			} else if polling.(bool) {
				// polling is still true, do nothing
				logging.Log.WithFields(logrus.Fields{"reason": "previous poll still in progress", "persistence_key": queryWithCallback.PersistenceKey}).Debug("skipping poll")
				return
			}

			// no poll in progress, so run the query and callback
			logging.Log.WithFields(logrus.Fields{"persistence_key": queryWithCallback.PersistenceKey}).Debug("polling")

			// attempt to query with the NextRecordsUrl first
			nextRecordsURL := p.getNextRecordsURL(queryWithCallback)
			if nextRecordsURL != "" {
				nextURLResponse, err := p.SfUtils.GetNextRecords(nextRecordsURL)
				if err != nil {
					// check if the NextRecordsUrl was not valid, return and
					// log if it was some other error
					// TODO could check the error better than this
					if !strings.Contains(err.Error(), "INVALID_QUERY_LOCATOR") {
						errorutils.LogOnErr(nil, "error getting next records", err)
						return
					}
				} else {
					if len(nextURLResponse.Records) > 0 {
						recordsJSON, err := json.Marshal(nextURLResponse.Records)
						if err != nil {
							errorutils.LogOnErr(nil, "error marshaling soql query response", err)
							return
						}
						p.handleSalesforceResponse(nextURLResponse, recordsJSON, queryWithCallback)
						return
					}
				}
			}
			// if we got here, then the NextRecordsUrl was empty, failed, or
			// had an empty reponse so query salesforce with the configured
			// query
			query, err := p.getPollQuery(queryWithCallback)
			if err != nil {
				errorutils.LogOnErr(nil, "error building query", err)
				return
			}
			logging.Log.WithFields(logrus.Fields{"query": query}).Debug("query")
			queryResponse, err := p.SfUtils.ExecuteSoqlQuery(query)
			if err != nil {
				errorutils.LogOnErr(nil, "error making soql query", err)
				return
			}

			if len(queryResponse.Records) > 0 {
				recordsJSON, err := json.Marshal(queryResponse.Records)
				if err != nil {
					errorutils.LogOnErr(nil, "error marshaling soql query response", err)
					return
				}
				newRecordsJSON, err := p.removeAlreadyQueriedRecords(recordsJSON, queryWithCallback)
				if err != nil {
					return
				}
				newRecordsLength := gjson.GetBytes(newRecordsJSON, "#").Int()
				if newRecordsLength > 0 {
					p.handleSalesforceResponse(queryResponse, newRecordsJSON, queryWithCallback)
				}
			}
		}(queryWithCallback)
	}
}

// removeAlreadyQueriedRecords checks a result for records that were already
// queried in the previous poll(). compares lastModifiedDate first to fail
// fast, then iterates over all results and compares with the saved IDs
func (p *LightningPoller) removeAlreadyQueriedRecords(recordsJSON []byte, queryWithCallback QueryWithCallback) (newRecordsJSON []byte, err error) {
	newRecordsJSON = recordsJSON
	resultLastModifiedDateString := getFinalLastModifiedDateStringFromJSON(recordsJSON)
	resultLastModifiedDate, err := getTimestampFromResultLastModifiedDate(resultLastModifiedDateString)
	if err != nil {
		errorutils.LogOnErr(nil, "error parsing LastModifiedDate", err)
		return
	}
	lastPosition := p.positions[queryWithCallback.PersistenceKey]
	positionLastModifiedDate := lastPosition.LastModifiedDate
	if resultLastModifiedDate.Equal(*positionLastModifiedDate) {
		// last modified dates are the same, check IDs and delete records that have matching IDs
		length := gjson.GetBytes(recordsJSON, "#").Int()
		// iterator for tracking index after deletes in json occur
		correctedIterator := 0
		for i := int64(0); i < length; i++ {
			recordID := gjson.GetBytes(recordsJSON, fmt.Sprintf("%d.Id", i)).String()
			if stringInSlice(recordID, lastPosition.PreviousRecordIDs) {
				newRecordsJSON, err = sjson.DeleteBytes(newRecordsJSON, fmt.Sprintf("%d", correctedIterator))
				if err != nil {
					errorutils.LogOnErr(nil, "error removing record from json", err)
					return
				}
				// decrement corrected iterator when a record is removed
				correctedIterator--
			}
			// increment the correct iterator each time
			correctedIterator++
		}
		newRecordsLength := gjson.GetBytes(newRecordsJSON, "#").Int()
		logging.Log.WithFields(logrus.Fields{"queried_records_total": length, "new_records_total": newRecordsLength}).Debug("removed already queried records")
		return
	}
	return
}

func (p *LightningPoller) handleSalesforceResponse(response pkg.SoqlResponse, recordsJSON []byte, queryWithCallback QueryWithCallback) {
	var err error
	savePosition := queryWithCallback.Callback(recordsJSON, err)
	if savePosition {
		positionErr := p.updatePosition(queryWithCallback.PersistenceKey, response, recordsJSON)
		if positionErr != nil {
			errorutils.LogOnErr(nil, "error updating position", positionErr)
		}
	}
}

func (p *LightningPoller) updatePosition(key string, response pkg.SoqlResponse, recordsJSON []byte) error {
	newPosition, err := getPositionFromResult(response, recordsJSON)
	if err != nil {
		return err
	}
	p.positions[key] = &newPosition
	// update saved position if persistence is enabled
	if p.config.PersistenceEnabled {
		err := p.setPosition(key, newPosition)
		if err != nil {
			return err
		}
	}
	logging.Log.WithFields(logrus.Fields{"lastModifiedDate": newPosition.LastModifiedDate, "persistence_key": key}).Debug("updated position")
	return nil
}

func getPositionFromResult(response pkg.SoqlResponse, recordsJSON []byte) (position Position, err error) {
	// save last modified timestamp from last record in response
	lastModifiedDate := getFinalLastModifiedDateStringFromJSON(recordsJSON)
	if lastModifiedDate == "" {
		logging.Log.WithFields(logrus.Fields{"json": string(recordsJSON)}).Debug("could not retrieve final last modified date from records json")
		err = errors.New("could not retrieve final last modified date from records")
		return
	}
	timestamp, timestampErr := getTimestampFromResultLastModifiedDate(lastModifiedDate)
	if timestampErr != nil {
		err = timestampErr
		return
	}
	position.LastModifiedDate = &timestamp
	// save all of the record IDs of the response
	lastQueriedIDs := []string{}
	gjsonIDresult := gjson.GetBytes(recordsJSON, "#.Id").Array()
	for _, result := range gjsonIDresult {
		lastQueriedIDs = append(lastQueriedIDs, result.String())
	}
	position.PreviousRecordIDs = lastQueriedIDs
	// save the next url
	position.NextURL = response.NextRecordsUrl
	return
}

func getFinalLastModifiedDateStringFromJSON(recordsJSON []byte) string {
	numRecords := gjson.GetBytes(recordsJSON, "#").Int()
	finalArrayIndex := numRecords - 1
	path := fmt.Sprintf("%d.LastModifiedDate", finalArrayIndex)
	finalLastModifiedDateResult := gjson.GetBytes(recordsJSON, path)
	finalLastModifiedDateString := finalLastModifiedDateResult.String()
	return finalLastModifiedDateString
}

// initConfig reads in config file and ENV variables if set.
func initConfig(queries []QueryWithCallback) (*RunConfig, error) {
	var cfgFile string
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".salesforce-lightning-poller" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".salesforce-lightning-poller")
	}
	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		logging.Log.WithField("file", viper.ConfigFileUsed()).Info("Using config file")
	}
	// setup env vars
	viper.SetEnvPrefix("LP")
	viper.AutomaticEnv() // read in environment variables that match
	viper.SetDefault("grant_type", "password")
	viper.SetDefault("poll_interval", "10s")
	viper.SetDefault("persistence_enabled", false)
	viper.SetDefault("persistence_path", ".")
	viper.SetDefault("api_version", "54.0")
	viper.SetDefault("startup_position_overrides", "")
	startupPositionOverridesAsStrings := viper.GetStringMapString("startup_position_overrides")
	logging.Log.WithFields(logrus.Fields{"startupPositionOverridesAsStrings": startupPositionOverridesAsStrings}).Debug("startup position overrides as strings")
	startupPositionOverrides, err := stringMapToTimeMap(startupPositionOverridesAsStrings)
	if err != nil {
		return nil, errorx.Decorate(err, "error initializing config, unable to parse startup_position_override")
	}
	logging.Log.WithFields(logrus.Fields{"startupPositionOverrides": startupPositionOverrides}).Debug("startup position overrides")
	config := &RunConfig{
		Queries:                  queries,
		Ticker:                   time.NewTicker(viper.GetDuration("poll_interval")),
		PersistenceEnabled:       viper.GetBool("persistence_enabled"),
		PersistencePath:          viper.GetString("persistence_path"),
		StartupPositionOverrides: startupPositionOverrides,
	}
	theValidator := validator.New()
	err = theValidator.Struct(config)
	if err != nil {
		errs := []error{}
		for _, err := range err.(validator.ValidationErrors) {
			errs = append(errs, errorx.IllegalArgument.New("invalid configuration: %s is a required configuration", err.Field()))
		}
		return nil, errorx.DecorateMany("error initializing config", errs...)
	}
	return config, nil
}

func stringMapToTimeMap(i map[string]string) (o map[string]time.Time, err error) {
	o = map[string]time.Time{}
	for k, v := range i {
		o[k], err = time.Parse(time.RFC3339, v)
		if err != nil {
			return nil, err
		}
	}
	return o, nil
}

func (p *LightningPoller) openBadgerDb(path string) error {
	db, err := badger.Open(badger.DefaultOptions(path))
	if err != nil {
		errorutils.LogOnErr(logging.Log.WithField("path", path), "error opening badger db", err)
	} else {
		p.db = db
	}
	return err
}

func (p *LightningPoller) closeBadgerDb() {
	err := p.db.Close()
	errorutils.LogOnErr(nil, "error closing badger database", err)
}

func (p *LightningPoller) getNextRecordsURL(queryWithCallback QueryWithCallback) string {
	return p.positions[queryWithCallback.PersistenceKey].NextURL
}

// getPollQuery is used to modify the base query according to configuration.
func (p *LightningPoller) getPollQuery(queryWithCallback QueryWithCallback) (string, error) {
	var builder strings.Builder
	builder.WriteString(queryWithCallback.Query())
	// query for last updated and update query based on stored timestamp
	persistenceKey := queryWithCallback.PersistenceKey
	currentPosition := p.positions[persistenceKey]
	operator := "where"
	// if there's a where clause, switch the operator to and so we append a condition instead of creating one
	if strings.Contains(strings.ToLower(builder.String()), operator) {
		operator = "and"
	}
	// use of rfc3339 is important here. SOQL uses + to indicate a space, so it parses out timestamp with + in them as a space, which is an invalid timestamp
	// and then it gets mad that the datetime isn't valid because it made it invalid by replacing the + (for the timezone) with a space.
	// if the time is not zero, use time - 2 seconds to make sure we never catch mid second updates
	//if position.LastModifiedDate.UTC() != zeroTime.UTC() {
	//	correctedTime := position.LastModifiedDate.Add(-2 * time.Second)
	//	position.LastModifiedDate = &correctedTime
	//}
	dateTimeString := getRfcFormattedUtcTimestampString(*currentPosition.LastModifiedDate)
	builder.WriteString(fmt.Sprintf(" %s LastModifiedDate >= %s order by LastModifiedDate, Id", operator, dateTimeString))
	return builder.String(), nil
}

// getPosition fetches the persisted position. If there is none, then it initializes to zero values
func (p *LightningPoller) getPosition(key []byte) (position *Position, err error) {
	err = p.db.View(func(txn *badger.Txn) error {
		item, getErr := txn.Get(key)
		if getErr != nil {
			return getErr
		}
		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &position)
		})
	})
	if err != nil {
		// if the key is not found, then return a new position with zero state
		if strings.Contains(err.Error(), "Key not found") {
			err = nil
			position = &Position{LastModifiedDate: &time.Time{}}
		}
	}
	return
}

func getRfcFormattedUtcTimestampString(timestamp time.Time) string {
	return timestamp.UTC().Format(time.RFC3339)
}

func (p *LightningPoller) setPosition(key string, position Position) error {
	positionBytes, err := json.Marshal(position)
	if err != nil {
		return err
	}
	err = p.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), positionBytes)
	})
	return err
}

func getTimestampFromResultLastModifiedDate(lastModifiedDate string) (timestamp time.Time, err error) {
	return time.Parse("2006-01-02T15:04:05.000+0000", lastModifiedDate)
}

func stringInSlice(str string, slice []string) bool {
	for _, v := range slice {
		if v == str {
			return true
		}
	}
	return false
}
