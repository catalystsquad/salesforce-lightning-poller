package pkg

import (
	"encoding/json"
	"fmt"
	"github.com/catalystsquad/app-utils-go/errorutils"
	"github.com/catalystsquad/app-utils-go/logging"
	"github.com/dgraph-io/badger/v3"
	"github.com/go-playground/validator/v10"
	"github.com/joomcode/errorx"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tidwall/gjson"
	"github.com/valyala/fasthttp"
	"net/http"
	"os"
	"strings"
	"time"
)

const orderByField = "LastModifiedDate"

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
}

type RunConfig struct {
	Domain             string              `json:"domain" validate:"required"`
	ClientId           string              `json:"client_id" validate:"required"`
	ClientSecret       string              `json:"client_secret" validate:"required"`
	Username           string              `json:"username" validate:"required"`
	Password           string              `json:"password" validate:"required"`
	GrantType          string              `json:"grant_type" validate:"required"`
	ApiVersion         string              `json:"api_version" validate:"required"`
	Queries            []QueryWithCallback `validate:"required"`
	Ticker             *time.Ticker
	AccessToken        string `json:"access_token"`
	PersistenceEnabled bool   `json:"persistence_enabled"`
	PersistencePath    string `json:"persistence_path"`
}

type QueryWithCallback struct {
	Query          func() string                  `json:"query" validate:"required"`
	PersistenceKey string                         `json:"persistenceKey"`
	Callback       func(result []byte, err error) `validate:"required"`
}

func NewLightningPoller(queries []QueryWithCallback) (*LightningPoller, error) {
	poller := &LightningPoller{}
	config, err := initConfig(queries)
	poller.config = config
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
			query, _ := p.getPollQuery(queryWithCallback)
			result, err := p.queryWithAuth(query)
			// if there is no error, update last modified
			if err == nil {
				newLastModified := getLastModifiedDateFromResult(result)
				if newLastModified != "" {
					err = p.setLastModified(queryWithCallback.PersistenceKey, newLastModified)
					if err != nil {
						errorutils.LogOnErr(logging.Log.WithField("query", query), "error updating last modified date", err)
					}
				}
			}
			queryWithCallback.Callback(result, err)
		}
		logging.Log.Debug("polling complete")
	} else {
		logging.Log.Debug("not polling because poll is currently in progress")
	}
}

func getLastModifiedDateFromResult(result []byte) string {
	lastModified := ""
	numRecords := gjson.GetBytes(result, "records.#").Int()
	if numRecords > 0 {
		path := fmt.Sprintf("records.%d.%s", numRecords-1, orderByField)
		lastModified = gjson.GetBytes(result, path).String()
	}
	return lastModified
}

func (p *LightningPoller) queryWithAuth(query string) ([]byte, error) {
	// check for empty access token, attempt to get token if it's empty
	if p.config.AccessToken == "" {
		err := p.getSalesforceCredentials()
		if err != nil {
			return nil, err
		}
	}
	// make query
	body, statusCode, err := p.query(query)
	// return on error
	if err != nil {
		return nil, errorx.Decorate(err, "error making query")
	}

	if statusCode == http.StatusOK {
		// return the body if we got a 200
		return body, nil
	} else if statusCode == http.StatusUnauthorized {
		// token is invalid or expired, authenticate and try again
		err = p.getSalesforceCredentials()
		body, statusCode, err = p.query(query)
		// return on error
		if err != nil {
			return nil, errorx.Decorate(err, "error making query")
		}
		if statusCode == http.StatusOK {
			return body, nil
		} else {
			// return an error
			return nil, errorx.Decorate(err, "unexpected status code: %d with body: %s", statusCode, body)
		}
	} else {
		// return an error
		return nil, errorx.Decorate(err, "unexpected status code: %d with body: %s", statusCode, body)
	}
}

func (p *LightningPoller) query(query string) ([]byte, int, error) {
	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	req.SetRequestURI(p.getQueryUrl(query))
	p.addAuthHeader(req)
	res := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(res)
	err := fasthttp.Do(req, res)
	return res.Body(), res.StatusCode(), err
}

func (p *LightningPoller) getSalesforceCredentials() error {
	body, statusCode, err := p.getSalesforceAccessToken()
	if err != nil {
		return errorx.Decorate(err, "error getting access token")
	}
	if statusCode != 200 {
		return errorx.Decorate(err, "error getting access token")
	}
	var creds salesforceCredentials
	err = json.Unmarshal(body, &creds)
	if err != nil {
		return errorx.Decorate(err, "error getting access token")
	}
	p.config.AccessToken = creds.AccessToken
	return nil
}

func (p *LightningPoller) getSalesforceAccessToken() ([]byte, int, error) {
	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	uri := p.getAuthUrl()
	req.SetRequestURI(uri)
	req.Header.SetMethod("POST")
	res := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(res)
	err := fasthttp.Do(req, res)
	return res.Body(), res.StatusCode(), err
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
	viper.SetDefault("api_version", "54.0")
	config := &RunConfig{
		Domain:       viper.GetString("domain"),
		ClientId:     viper.GetString("client_id"),
		ClientSecret: viper.GetString("client_secret"),
		Username:     viper.GetString("username"),
		Password:     viper.GetString("password"),
		GrantType:    viper.GetString("grant_type"),
		ApiVersion:   viper.GetString("api_version"),
		Queries:      queries,
		Ticker:       time.NewTicker(viper.GetDuration("poll_interval")),
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

// getQueryUrl gets a formatted url to the soql query endpoint
func (p *LightningPoller) getQueryUrl(query string) string {
	formattedQuery := strings.Replace(query, " ", "+", -1)
	return fmt.Sprintf("%s/services/data/v%s/query?q=%s", p.getBaseUrl(), p.config.ApiVersion, formattedQuery)
}

// getAuthUrl gets a formatted url to the token endpoint
func (p *LightningPoller) getAuthUrl() string {
	return fmt.Sprintf("%s/services/oauth2/token?client_id=%s&client_secret=%s&username=%s&password=%s&grant_type=%s", p.getBaseUrl(), p.config.ClientId, p.config.ClientSecret, p.config.Username, p.config.Password, p.config.GrantType)
}

// getBaseUrl gets a base url using the configured domain
func (p *LightningPoller) getBaseUrl() string {
	return fmt.Sprintf("https://%s", p.config.Domain)
}

// addAuthHeader adds the access token from the config to the request
func (p *LightningPoller) addAuthHeader(req *fasthttp.Request) {
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", p.config.AccessToken))
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

// getPollQuery is used to modify the query if persistence is enabled so that the query
func (p *LightningPoller) getPollQuery(queryWithCallback QueryWithCallback) (string, error) {
	query := queryWithCallback.Query()
	// if persistence is disabled just return the query as is
	if !p.config.PersistenceEnabled {
		return query, nil
	} else {
		// query for last updated and update query based on stored timestamp
		persistenceKey := queryWithCallback.PersistenceKey
		LastModified, err := p.getLastModified([]byte(persistenceKey))
		if err != nil {
			return "", err
		}
		if LastModified != "" {
			operator := "where"
			// if there's a where clause, switch the operator to and so we append a condition instead of creating one
			if strings.Contains(strings.ToLower(query), operator) {
				operator = "and"
			}
			return fmt.Sprintf("%s %s %s > %s order by %s", query, operator, orderByField, LastModified, orderByField), nil
		} else {
			return query, nil
		}
	}
}

func (p *LightningPoller) getLastModified(key []byte) (LastModified string, err error) {
	err = p.db.View(func(txn *badger.Txn) error {
		item, getErr := txn.Get(key)
		if getErr != nil {
			return getErr
		}
		item.Value(func(val []byte) error {
			LastModified = string(val)
			return nil
		})
		return nil
	})
	return
}

func (p *LightningPoller) setLastModified(key string, value string) error {
	err := p.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), []byte(value))
	})
	return err
}
