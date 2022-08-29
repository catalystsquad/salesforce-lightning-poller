package pkg

import (
	"encoding/json"
	"fmt"
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
	"os"
	"strings"
	"sync"
	"time"
)

type Position struct {
	Offset           int
	LastModifiedDate *time.Time
}

type LightningPoller struct {
	config    *RunConfig
	pollMap   *sync.Map
	db        *badger.DB
	SfUtils   *pkg.SalesforceUtils
	positions map[string]*Position
}

type RunConfig struct {
	Queries            []QueryWithCallback `validate:"required"`
	Ticker             *time.Ticker
	PersistenceEnabled bool   `json:"persistence_enabled"`
	PersistencePath    string `json:"persistence_path"`
	Limit              int    `json:"limit" validate:"gte=1,lte=1000"` // limit must be between 1-1000
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
	// load position for each query based on perisstence key
	for _, query := range p.config.Queries {
		key := query.PersistenceKey
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
			} else {
				// no poll in progress, so run the query and callback
				logging.Log.WithFields(logrus.Fields{"persistence_key": queryWithCallback.PersistenceKey}).Debug("polling")
				query, err := p.getPollQuery(queryWithCallback)
				if err != nil {
					errorutils.LogOnErr(nil, "error building query", err)
					return
				}
				logging.Log.WithFields(logrus.Fields{"query": query}).Debug("query")
				response, err := p.SfUtils.ExecuteSoqlQuery(query)
				if err != nil {
					errorutils.LogOnErr(nil, "error making soql query", err)
				} else {
					if len(response.Records) > 0 {
						recordsJson, jsonErr := json.Marshal(response.Records)
						if jsonErr != nil {
							errorutils.LogOnErr(nil, "error making soql query", err)
						} else {
							savePosition := queryWithCallback.Callback(recordsJson, err)
							if savePosition {
								positionErr := p.updatePosition(queryWithCallback.PersistenceKey, recordsJson)
								if positionErr != nil {
									errorutils.LogOnErr(nil, "error updating position", positionErr)
								}
							}
						}
					}
				}
			}
		}(queryWithCallback)
	}
}

func (p *LightningPoller) updatePosition(key string, recordsJson []byte) error {
	newPosition, err := getNewPositionFromResult(recordsJson)
	if err != nil {
		return err
	}
	position := p.positions[key]
	// if the date is the same, add the offsets. This would happen if you got several pages with the same date. If you
	/// don't combine the offsets then you'll just get the same page of data over and over forever.
	if position.LastModifiedDate.Equal(*newPosition.LastModifiedDate) {
		position.Offset += newPosition.Offset
	} else {
		// date is different, replace the position entirely
		position.LastModifiedDate = newPosition.LastModifiedDate
		position.Offset = newPosition.Offset
	}
	// update saved position if persistence is enabled
	if p.config.PersistenceEnabled {
		err := p.setPosition(key, *position)
		if err != nil {
			return err
		}
	}
	logging.Log.WithFields(logrus.Fields{"lastModifiedDate": position.LastModifiedDate, "offset": position.Offset}).Debug("updated position")
	return nil
}

func getNewPositionFromResult(recordsJson []byte) (position Position, err error) {
	numRecords := gjson.GetBytes(recordsJson, "#").Int()
	finalArrayIndex := numRecords - 1
	path := fmt.Sprintf("%d.LastModifiedDate", finalArrayIndex)
	finalLastModifiedDateResult := gjson.GetBytes(recordsJson, path)
	finalLastModifiedDateString := finalLastModifiedDateResult.String()
	// loop backwards until we hit a different date, incrementing the offset each time.
	for i := finalArrayIndex; i >= 0; i-- {
		// break if the date is different
		path := fmt.Sprintf("%d.LastModifiedDate", i)
		if gjson.GetBytes(recordsJson, path).String() != finalLastModifiedDateString {
			break
		}
		// date is the same, increment offset
		position.Offset++
	}
	timestamp, timestampErr := getTimestampFromResultLastModifiedDate(finalLastModifiedDateString)
	if timestampErr != nil {
		err = timestampErr
		return
	}
	position.LastModifiedDate = &timestamp
	return
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
	config := &RunConfig{
		Queries:            queries,
		Ticker:             time.NewTicker(viper.GetDuration("poll_interval")),
		PersistenceEnabled: viper.GetBool("persistence_enabled"),
		PersistencePath:    viper.GetString("persistence_path"),
		Limit:              viper.GetInt("limit"),
	}
	theValidator := validator.New()
	err := theValidator.Struct(config)
	if err != nil {
		errs := []error{}
		for _, err := range err.(validator.ValidationErrors) {
			errs = append(errs, errorx.IllegalArgument.New("invalid configuration: %s is a required configuration", err.Field()))
		}
		return nil, errorx.DecorateMany("error initializing config", errs...)
	}
	return config, nil
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
	builder.WriteString(fmt.Sprintf(" %s LastModifiedDate >= %s order by LastModifiedDate, Id limit %d offset %d", operator, dateTimeString, p.config.Limit, currentPosition.Offset))
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
