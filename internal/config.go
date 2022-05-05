package internal

import (
	"encoding/json"
	"github.com/catalystsquad/app-utils-go/errorutils"
	"github.com/catalystsquad/app-utils-go/logging"
	"github.com/go-playground/validator/v10"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type runConfig struct {
	Domain       string `json:"domain" validate:"required"`
	ClientId     string `json:"client_id" validate:"required"`
	ClientSecret string `json:"client_secret" validate:"required"`
	Username     string `json:"username" validate:"required"`
	Password     string `json:"password" validate:"required"`
	GrantType    string `json:"grant_type" validate:"required"`
	PollInterval string `json:"poll_interval" validate:"required"`
	ApiVersion   string `json:"api_version" validate:"required"`
	Queries      []queryWithCallback
	AccessToken  string `json:"access_token"`
}

type queryWithCallback struct {
	Query    string              `json:"query" validate:"required"`
	Callback func(result []byte) `validate:"required"`
}

func Validate() *runConfig {
	var config *runConfig
	// unmarshal can't be called in init(), it must be called after init() has taken place. Init the settings.
	settingsBytes, err := json.Marshal(viper.AllSettings())
	errorutils.PanicOnErr(nil, "error marshalling viper settings to json", err)
	logging.Log.WithFields(logrus.Fields{"settingsBytes": string(settingsBytes)}).Info("validate")
	err = json.Unmarshal(settingsBytes, &config)
	errorutils.PanicOnErr(nil, "error unmarshalling viper settings to runConfiguration struct", err)
	theValidator := validator.New()
	err = theValidator.Struct(config)
	if err != nil {
		for _, err := range err.(validator.ValidationErrors) {
			logging.Log.Errorf("invalid command: %s is a required configuration, use -h for help", err.Field())
		}
		return nil
	}
	return config
}
