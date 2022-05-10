package main

import (
	"github.com/catalystsquad/app-utils-go/errorutils"
	"github.com/catalystsquad/app-utils-go/logging"
	"github.com/catalystsquad/salesforce-lightning-poller/pkg"
	"github.com/sirupsen/logrus"
)

func main() {
	queries := []pkg.QueryWithCallback{
		{
			Query: func() string {
				return "select fields(all) from Property__c"
			},
			PersistenceKey: "property__c",
			Callback: func(result []byte, err error) {
				logging.Log.WithFields(logrus.Fields{"result": string(result)}).Info("query callback")
			},
		},
	}
	poller, err := pkg.NewLightningPoller(queries)
	errorutils.PanicOnErr(nil, "error creating poller", err)
	poller.Run()
}
