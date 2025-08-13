package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/cloudfoundry-community/scs-broker/broker"
	"github.com/cloudfoundry-community/scs-broker/config"
	"github.com/pivotal-cf/brokerapi"
)

var brokerLogger lager.Logger
// (legacy) httpTransport removed; add back if artifact transport needed in future

// Version is overridden at build time via: -ldflags "-X main.Version=1.1.3"
var Version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(Version)
		return
	}
	brokerLogger = lager.NewLogger("scs-broker")
	brokerLogger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))
	brokerLogger.RegisterSink(lager.NewWriterSink(os.Stderr, lager.ERROR))

	brokerConf, err := config.ParseConfig()
	if err != nil {
		brokerLogger.Fatal("Reading config from env", err, lager.Data{
			"broker-config-environment-variable": config.ConfigEnvVarName,
		})
	}

	brokerLogger.Info("starting", lager.Data{"version": Version})

	serviceBroker := &broker.SCSBroker{
		Config: brokerConf,
		Logger: brokerLogger,
	}

	brokerCredentials := brokerapi.BrokerCredentials{
		Username: brokerConf.Auth.Username,
		Password: brokerConf.Auth.Password,
	}

	brokerAPI := brokerapi.New(serviceBroker, brokerLogger, brokerCredentials)
	http.Handle("/", brokerAPI)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	brokerLogger.Info("listening", lager.Data{"port": port})
	brokerLogger.Fatal("http-listen", http.ListenAndServe("0.0.0.0:"+port, nil))
}
