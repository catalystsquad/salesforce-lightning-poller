# Salesforce Lightning Poller
We created the lightning poller because we didn't like the cometd approach. Configuration is handled via environment variables and a simple struct.
## Persistence
If `LP_PERSISTENCE_ENABLED` is set to true, then the poller will persist state on disk, as well as append a where clause and an order by clause. An example query when persistence is enabled would be `select fields(all) from MyObject__c where LastModifiedDate >= 2021-03-10T13:56:52Z order by LastModifiedDate, Id limit 10 offset 1`. The poller tracks the most recent modified date that it's encountered, as well as the number of times it was previously encountered, to set the LastModifiedDate where clause, and the offset. We recommend using this mode because it lets the poller do the heavy lifting for you so that your queries can be simple, such as `select fields(all) from MyObject__c` without having to handle the other clauses, limits, offsets, etc. If you don't use persistence then you must handle that on your own between the query() and callback() functions.
### PersistenceKey
If `LP_PERSISTENCE_ENABLED` is true, then you must also configure the `PersistenceKey` for each `QueryWithCallback` object. It must be unique among your list of `QueryWithCallback`. The poller uses this as the key to persist data for a given query.
## Usage Example
To use the poller, define an array of QueryWithCallback structs. These structs have a query function to execute, and a callback function that gets called after the execution with the result, and an error. For simple use cases the query function can return a string. For more complex use cases you may want to store state on disk, in memory, in a database, or do somethign else before running the query or generating the query.

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
            Query: func() string {
				return "select fields(all) from Account limit 1"
			},
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
|LP_PERSISTENCE_ENABLED|no|Enable persistence and ordering to simplify queries and recovery. Defaults to `false`|
|LP_PERSISTENCE_PATH|no|Path to disk location to store data. Defaults to `.`|