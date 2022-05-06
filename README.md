# Salesforce Lightning Poller
We created the lightning poller because we didn't like the cometd approach. Configuration is handled via environment variables and a simple struct.
## Usage Example
To use the poller, define an array of QueryWithCallback structs. These structs have a query string to execute, and a callback function that gets called after the execution with the result, and an error.

Create a new poller using the queries array, and then call `Run()` on the poller. Each query and callback will be executed on an interval. Authentication is done using the username and password to get an access token, the token is then used to run the queries. If the token expires, a new token is retrieved, so this can run for a long time without worrying about authentication problems. Spaces in the query are replaced with `+` before it's sent, this is done to make it easier to read and use. You can use `+` in your query if you'd like.
```go
package main

import (
    "github.com/catalystsquad/app-utils-go/errorutils"
    "github.com/catalystsquad/app-utils-go/logging"
    "github.com/catalystsquad/salesforce-lightning-poller/pkg"
)

func main() {
    queries := []pkg.QueryWithCallback{
        {
            Query: "select fields(all) from Account limit 1",
            Callback: func(result []byte, err error) {
                if err != nil {
                  logging.Log.WithError(err).Error("error executing query")
                } else {
                  logging.Log.WithField("result", string(result)).Info("executed query")
                  // your code here
                }
            },
        },
    }
    poller, err := pkg.NewLightningPoller(queries)
    errorutils.PanicOnErr(nil, "error creating poller", err)
    poller.Run()
}
```
## Configuration
Configuration is handled by environment variables prefixed with `LP_` to avoid conflicts
| name |required| purpose |
|--|--|--|
|LP_DOMAIN|yes|Set the salesforce domain, i.e. mydomain.my.salesforce.com |
|LP_CLIENT_ID|yes|Set the connected app client id|
|LP_CLIENT_SECRET|yes|Set the connected app client secret|
|LP_USERNAME|yes|User to authenticate as|
|LP_PASSWORD|yes|Password to authenticate with
|LP_GRANT_TYPE|no|Grant type, defaults to `password`, we advise not setting this and letting it use the default|
|LP_API_VERSION|no|Salesforce api version to use, defaults to 54.0|
|LP_POLL_INTERVAL|no|How often to poll for data, defaults to `10s`|