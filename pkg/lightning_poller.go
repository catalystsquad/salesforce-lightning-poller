package pkg

import (
	"encoding/json"
	"fmt"
	"github.com/catalystsquad/app-utils-go/logging"
	"github.com/go-playground/validator/v10"
	"github.com/joomcode/errorx"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/valyala/fasthttp"
	"net/http"
	"os"
	"strings"
	"time"
)

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
}

type RunConfig struct {
	Domain       string              `json:"domain" validate:"required"`
	ClientId     string              `json:"client_id" validate:"required"`
	ClientSecret string              `json:"client_secret" validate:"required"`
	Username     string              `json:"username" validate:"required"`
	Password     string              `json:"password" validate:"required"`
	GrantType    string              `json:"grant_type" validate:"required"`
	ApiVersion   string              `json:"api_version" validate:"required"`
	Queries      []QueryWithCallback `validate:"required"`
	Ticker       *time.Ticker
	AccessToken  string `json:"access_token"`
}

type QueryWithCallback struct {
	Query    string                         `json:"query" validate:"required"`
	Callback func(result []byte, err error) `validate:"required"`
}

func NewLightningPoller(queries []QueryWithCallback) (*LightningPoller, error) {
	poller := &LightningPoller{}
	config, err := initConfig(queries)
	poller.config = config
	return poller, err
}

func (p *LightningPoller) Run() {
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
			result, err := p.queryWithAuth(queryWithCallback.Query)
			queryWithCallback.Callback(result, err)
		}
		logging.Log.Debug("polling complete")
	} else {
		logging.Log.Debug("not polling because poll is currently in progress")
	}
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
