package main

import (
	"github.com/catalystsquad/app-utils-go/errorutils"
	"github.com/catalystsquad/app-utils-go/logging"
	"github.com/catalystsquad/salesforce-lightning-poller/pkg"
	pkg2 "github.com/catalystsquad/salesforce-utils/pkg"
	"github.com/sirupsen/logrus"
)

func main() {
	queries := []pkg.QueryWithCallback{
		{
			Query: func() string {
				return "select fields(all) from Property__c"
			},
			PersistenceKey: "property__c",
			Callback: func(result []byte, err error) bool {
				logging.Log.WithFields(logrus.Fields{"result": string(result)}).Info("query callback")
				return true
			},
		},
	}
	poller, err := pkg.NewLightningPoller(queries, pkg2.Config{}, nil, nil)
	errorutils.PanicOnErr(nil, "error creating poller", err)
	poller.Run()
}
