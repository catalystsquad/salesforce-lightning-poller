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
	"time"
)

var zeroTime = time.Time{}

type Position struct {
	Offset           int
	LastModifiedDate *time.Time
}

// salesforceCredentials represents the response from salesforce's /services/oauth2/token endpoint to get an access token
type salesforceCredentials struct {
	AccessToken string `json:"access_token"`
	InstanceUrl string `json:"instance_url"`
	Id          string `json:"id"`
	TokenType   string `json:"token_type"`
	IssuedAt    int    `json:"issued_at,string"`
	Signature   string `json:"signature"`
}

type LightningPoller struct {
	config  *RunConfig
	polling bool
	db      *badger.DB
	sfUtils *pkg.SalesforceUtils
}

type RunConfig struct {
	Queries            []QueryWithCallback `validate:"required"`
	Ticker             *time.Ticker
	PersistenceEnabled bool   `json:"persistence_enabled"`
	PersistencePath    string `json:"persistence_path"`
	Limit              int    `json:"limit" validate:"gte=1,lte=1000"` // limit must be between 1-1000
}

type QueryWithCallback struct {
	Query          func() string                  `json:"query" validate:"required"`
	PersistenceKey string                         `json:"persistenceKey"`
	Callback       func(result []byte, err error) `validate:"required"`
}

func NewLightningPoller(queries []QueryWithCallback, sfConfig pkg.Config) (*LightningPoller, error) {
	poller := &LightningPoller{}
	config, err := initConfig(queries)
	if err != nil {
		return nil, err
	}
	poller.config = config
	poller.sfUtils, err = pkg.NewSalesforceUtils(true, sfConfig)
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
	for range p.config.Ticker.C {
		p.poll()
	}
}

func (p *LightningPoller) poll() {
	if !p.polling {
		p.polling = true
		defer func() { p.polling = false }()
		logging.Log.Debug("polling")
		for _, queryWithCallback := range p.config.Queries {
			query, err := p.getPollQuery(queryWithCallback)
			if err != nil {
				errorutils.LogOnErr(nil, "error building query", err)
				continue
			}
			logging.Log.WithFields(logrus.Fields{"query": query}).Debug("query")
			response, err := p.sfUtils.ExecuteSoqlQuery(query)
			if err != nil {
				errorutils.LogOnErr(nil, "error making soql query", err)
			} else {
				recordsJson, jsonErr := json.Marshal(response.Records)
				if jsonErr != nil {
					errorutils.LogOnErr(nil, "error making soql query", err)
				} else {
					// if there is no error, update last modified
					newPosition, newPositionErr := getNewPositionFromResult(recordsJson)
					if newPositionErr != nil {
						errorutils.LogOnErr(logging.Log.WithField("query", query), "error parsing LastModifiedDate from result", err)
						continue
					}
					if newPosition.LastModifiedDate != nil {
						err = p.setPosition(queryWithCallback.PersistenceKey, newPosition)
						if err != nil {
							errorutils.LogOnErr(logging.Log.WithField("query", query), "error updating last modified date", err)
							continue
						}
					}
					queryWithCallback.Callback(recordsJson, err)
				}
			}
		}
		logging.Log.Debug("polling complete")
	} else {
		logging.Log.Debug("not polling because poll is currently in progress")
	}
}

func getNewPositionFromResult(result []byte) (position Position, err error) {
	numRecords := gjson.GetBytes(result, "records.#").Int()
	finalArrayIndex := numRecords - 1
	if numRecords > 0 {
		path := fmt.Sprintf("records.%d.LastModifiedDate", finalArrayIndex)
		finalLastModifiedDateResult := gjson.GetBytes(result, path)
		finalLastModifiedDateString := finalLastModifiedDateResult.String()
		// loop backwards until we hit a different date, incrementing the offset each time.
		for i := finalArrayIndex; i >= 0; i-- {
			// break if the date is different
			path := fmt.Sprintf("records.%d.LastModifiedDate", i)
			if gjson.GetBytes(result, path).String() != finalLastModifiedDateString {
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
	}
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
	// if persistence is disabled just return the query as is
	if !p.config.PersistenceEnabled {
		return builder.String(), nil
	} else {
		// query for last updated and update query based on stored timestamp
		persistenceKey := queryWithCallback.PersistenceKey
		position, err := p.getPosition([]byte(persistenceKey))
		if err != nil && !strings.Contains("Key not found", err.Error()) {
			return "", err
		}
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
		dateTimeString := getRfcFormattedUtcTimestampString(*position.LastModifiedDate)
		builder.WriteString(fmt.Sprintf(" %s LastModifiedDate >= %s order by LastModifiedDate, Id limit %d offset %d", operator, dateTimeString, p.config.Limit, position.Offset))
		return builder.String(), nil
	}
}

// getPosition fetches the persisted position. If there is none, then it initializes to zero values
func (p *LightningPoller) getPosition(key []byte) (position Position, err error) {
	err = p.db.View(func(txn *badger.Txn) error {
		item, getErr := txn.Get(key)
		if getErr != nil {
			return getErr
		}
		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &position)
		})
	})
	// if there is no stored date, use a zero date
	if position.LastModifiedDate == nil {
		position.LastModifiedDate = &time.Time{}
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
