package cmd

import (
	"github.com/catalystsquad/app-utils-go/logging"
	"github.com/catalystsquad/salesforce-lightning-poller/internal"
	"github.com/spf13/viper"
	"time"

	"github.com/catalystsquad/app-utils-go/errorutils"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Runs the salesforce lightning poller",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		// validate config, exit if config is invalid
		config := internal.Validate()
		if config == nil {
			return
		}
		// run ticker
		duration, err := time.ParseDuration(config.PollInterval)
		errorutils.PanicOnErr(nil, "poll_interval is not a valid duration", err)
		ticker := time.NewTicker(duration)
		polling := false
		for range ticker.C {
			// only poll if a poll is not currently in progress
			if !polling {
				polling = true
				go func() {
					// defer setting polling to false after poll has completed
					defer func() {
						polling = false
						logging.Log.Debug("poll complete")
					}()
					internal.Poll(config)
				}()
			} else {
				logging.Log.Debug("not polling because poll is currently in progress")
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.PersistentFlags().String("domain", "", "salesforce domain to use for authentication, such as `MyDomainName.my.salesforce.com`")
	runCmd.PersistentFlags().String("client_id", "", "client id to use for authentication")
	runCmd.PersistentFlags().String("client_secret", "", "client secret to use for authentication")
	runCmd.PersistentFlags().String("username", "", "username to use for authentication")
	runCmd.PersistentFlags().String("password", "", "password to use for authentication")
	runCmd.PersistentFlags().String("grant_type", "password", "grant type to use for authentication")
	runCmd.PersistentFlags().Duration("poll_interval", 10*time.Second, "how often to poll for data")
	runCmd.PersistentFlags().String("api_version", "54.0", "salesforce api version to use")
	err := viper.BindPFlags(runCmd.PersistentFlags())
	errorutils.PanicOnErr(nil, "error getting configuration", err)
}
