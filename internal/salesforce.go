package internal

import (
	"encoding/json"
	"fmt"
	"github.com/catalystsquad/app-utils-go/errorutils"
	"github.com/catalystsquad/app-utils-go/logging"
	"github.com/joomcode/errorx"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"net/http"
	"strings"
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

// Poll runs queries and calls callbacks. It handles authentication if the token is expired.
func Poll(config *runConfig) {
	result := queryWithAuth(config, "select fields(all) from Property__c limit 1")
	logging.Log.WithFields(logrus.Fields{"result": string(result)}).Info("got result")
}

// queryWithAuth attempts to query first. Upon receiving a 401 it attempts to authenticate with salesforce, if auth
// is successful then it tries to query again with the access token received from the authentication call
func queryWithAuth(config *runConfig, queryString string) (result []byte) {
	// make query
	body, statusCode, err := query(config, queryString)
	// panic on error
	errorutils.PanicOnErr(nil, "error executing soql query", err)
	if statusCode == http.StatusOK {
		// return body on 200
		return body
	} else if statusCode == http.StatusUnauthorized {
		// authenticate and try again on 403
		credentials, err := getSalesforceCredentials(config)
		errorutils.PanicOnErr(nil, "error getting salesforce credentials", err)
		config.AccessToken = credentials.AccessToken
		// make query with new credentials
		body, statusCode, err = query(config, queryString)
		errorutils.PanicOnErr(nil, "error executing soql query", err)
		if statusCode == http.StatusOK {
			// return body on 200
			return body
		} else {
			// panic on anything else, no auth this time since we just tried it
			errorutils.PanicOnErr(nil, "unexpected status code running soql query after authentication", errorx.IllegalState.New("unexpected status code: %d with body: %s", statusCode, body))
			return
		}
	} else {
		// panic on anything else
		errorutils.PanicOnErr(nil, "unexpected status code", errorx.IllegalState.New("unexpected status code: %d with body: %s", statusCode, body))
		return
	}
}

// query makes a request to salesforce's /query endpoint to execute a soql query
func query(config *runConfig, query string) ([]byte, int, error) {
	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	req.SetRequestURI(getQueryUrl(config, query))
	addAuthHeader(req, config)
	res := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(res)
	err := fasthttp.Do(req, res)
	if err != nil {
		return nil, 0, err
	}
	return res.Body(), res.StatusCode(), nil
}

// getSalesforceCredentials uses the configured credentials to get an access token
func getSalesforceCredentials(config *runConfig) (salesforceCredentials, error) {
	var creds salesforceCredentials
	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	uri := getAuthUrl(config)
	req.SetRequestURI(uri)
	req.Header.SetMethod("POST")
	res := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(res)
	err := fasthttp.Do(req, res)
	// return any errors
	if err != nil {
		errorutils.LogOnErr(nil, "error setting access token", err)
		return creds, err
	}
	statusCode := res.StatusCode()
	body := res.Body()
	// ensure we got 200 response
	if statusCode != http.StatusOK {
		entry := logging.Log.WithFields(logrus.Fields{"status_code": statusCode, "body": string(body)})
		errorutils.LogOnErr(entry, "unexpected status code attempting to set access token", errorx.IllegalState.New("unexpected status code"))
	}
	// unmarshal response body to struct
	err = json.Unmarshal(body, &creds)
	if err != nil {
		errorutils.LogOnErr(nil, "error unmarshalling access token response to struct", err)
		return creds, err
	}
	// ensure we actually got an access token
	if creds.AccessToken == "" {
		err = errorx.IllegalState.New("access token should not be empty")
		errorutils.LogOnErr(nil, "access token should not be empty", err)
		return creds, err
	}
	return creds, nil
}

// getQueryUrl gets a formatted url to the soql query endpoint
func getQueryUrl(config *runConfig, query string) string {
	formattedQuery := strings.Replace(query, " ", "+", -1)
	return fmt.Sprintf("%s/services/data/v%s/query?q=%s", getBaseUrl(config), config.ApiVersion, formattedQuery)
}

// getAuthUrl gets a formatted url to the token endpoint
func getAuthUrl(config *runConfig) string {
	return fmt.Sprintf("%s/services/oauth2/token?client_id=%s&client_secret=%s&username=%s&password=%s&grant_type=%s", getBaseUrl(config), config.ClientId, config.ClientSecret, config.Username, config.Password, config.GrantType)
}

// getBaseUrl gets a base url using the configured domain
func getBaseUrl(config *runConfig) string {
	return fmt.Sprintf("https://%s", config.Domain)
}

// addAuthHeader adds the access token from the config to the request
func addAuthHeader(req *fasthttp.Request, config *runConfig) {
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", config.AccessToken))
}
