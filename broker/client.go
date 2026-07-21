package broker

import (
	"code.cloudfoundry.org/cli/v8/api/cloudcontroller/ccv3"
	clients "github.com/cloudfoundry-community/go-cf-clients-helper/v2"
	"github.com/cloudfoundry-community/go-uaa"
	cfclient "github.com/cloudfoundry/go-cfclient/v3/client"
	cfconfig "github.com/cloudfoundry/go-cfclient/v3/config"
)

func (broker *SCSBroker) GetClient() (*ccv3.Client, error) {

	config := clients.Config{
		Endpoint:          broker.Config.CfConfig.ApiUrl,
		SkipSslValidation: broker.Config.CfConfig.SkipSslValidation,
		User:              broker.Config.CfConfig.CfUsername,
		Password:          broker.Config.CfConfig.CfPassword,
	}

	session, err := clients.NewSession(config)
	if err != nil {
		broker.Logger.Error("broker.Client (go-cf-clients-helper) clients.NewSession()", err)
		return nil, err
	}
	return session.V3(), err
}

func (broker *SCSBroker) GetCommunity() (*cfclient.Client, error) {
	opts := []cfconfig.Option{
		cfconfig.UserPassword(broker.Config.CfConfig.CfUsername, broker.Config.CfConfig.CfPassword),
	}
	if broker.Config.CfConfig.SkipSslValidation {
		opts = append(opts, cfconfig.SkipTLSValidation())
	}

	cfg, err := cfconfig.New(broker.Config.CfConfig.ApiUrl, opts...)
	if err != nil {
		broker.Logger.Error("broker.GetCommunity (go-cfclient/v3) config.New()", err)
		return nil, err
	}

	return cfclient.New(cfg)
}

func (broker *SCSBroker) GetUaaClient() (*uaa.API, error) {

	cf, err := broker.GetClient()
	if err != nil {
		broker.Logger.Error("broker.Client broker.GetClient()", err)
		return nil, err
	}
	info, _, err := cf.GetRoot()
	if err != nil {
		broker.Logger.Error("broker.Client (go-cfclient:) cf.GetClient()", err)
		return nil, err
	}

	uaaClient, err := uaa.New(info.UAA(), uaa.WithClientCredentials(broker.Config.CfConfig.UaaClientID, broker.Config.CfConfig.UaaClientSecret, uaa.JSONWebToken), uaa.WithSkipSSLValidation(broker.Config.CfConfig.SkipSslValidation))
	if err != nil {
		broker.Logger.Error("broker.Client: uaa.New()", err)
		return nil, err
	}
	return uaaClient, err
}
