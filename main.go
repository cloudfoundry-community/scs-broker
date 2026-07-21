package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"code.cloudfoundry.org/brokerapi/v13"
	"code.cloudfoundry.org/lager/v3"
	"github.com/cloudfoundry-community/scs-broker/broker"
	"github.com/cloudfoundry-community/scs-broker/config"
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

	// brokerapi v11+ takes a *slog.Logger; bridge the lager logger through
	// lager/v3's slog.Handler so broker-internal logging stays lager-native.
	brokerAPI := brokerapi.New(serviceBroker, slog.New(lager.NewHandler(brokerLogger)), brokerCredentials)
	http.Handle("/", brokerAPI)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	brokerLogger.Info("listening", lager.Data{"port": port})
	brokerLogger.Fatal("http-listen", http.ListenAndServe("0.0.0.0:"+port, nil))
}
