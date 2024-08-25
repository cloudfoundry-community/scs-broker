package broker

import (
	"context"
	"fmt"

	"github.com/cloudfoundry-community/go-uaa"
	"github.com/cloudfoundry-community/scs-broker/broker/utilities"
	brokerapi "github.com/pivotal-cf/brokerapi/domain"
)

func (broker *SCSBroker) Bind(ctx context.Context, instanceID, bindingID string, details brokerapi.BindDetails, asyncAllowed bool) (brokerapi.Binding, error) {
	binding := brokerapi.Binding{}

	broker.Logger.Info("broker.Bind: broker.GetUAAClient()")
	api, err := broker.GetUaaClient()
	if err != nil {
		broker.Logger.Error("broker.Bind broker.GetUaaClient()", err)
		return binding, err
	}

	clientId := utilities.MakeClientIdForBinding(details.ServiceID, bindingID)
	password := utilities.GenClientPassword()

	client := uaa.Client{
		ClientID:             clientId,
		AuthorizedGrantTypes: []string{"client_credentials"},
		Authorities:          []string{fmt.Sprintf("%s.%v.read", details.ServiceID, instanceID)},
		DisplayName:          clientId,
		ClientSecret:         password,
	}

	broker.Logger.Info("broker.Bind: api.CreateClient(")
	_, err = api.CreateClient(client)
	if err != nil {
		broker.Logger.Error("broker.Bind api.CreateClient()", err)
		return binding, err
	}

	broker.Logger.Info("broker.Bind: GetClient")
	cfClient, err := broker.GetClient()
	if err != nil {
		broker.Logger.Error("broker.Bind broker.GetClient()", err)
		return binding, err
	}

	broker.Logger.Info("broker.Bind: Get Info")
	info, _, _, err := cfClient.GetInfo()
	if err != nil {
		broker.Logger.Error("broker.Bind cfClient.GetInfo()", err)
		return binding, err
	}

	broker.Logger.Info("broker.Bind: cfClient.GetApplicationByNameAndSpace()")
	app, _, err := cfClient.GetApplicationByNameAndSpace(utilities.MakeAppName(details.ServiceID, instanceID), broker.Config.InstanceSpaceGUID)
	if err != nil {
		broker.Logger.Error("broker.Bind cfClient.GetApplicationByNameAndSpace()", err)
		return binding, err
	}

	broker.Logger.Info("broker.Bind: GetApplicationRoutes")
	routes, _, err := cfClient.GetApplicationRoutes(app.GUID)
	if err != nil {
		broker.Logger.Error("broker.Bind cfClient.GetApplicationRoutes()", err)
		return binding, err
	}

	broker.Logger.Info("broker.Bind: Building binding Credentials")
	binding.Credentials = map[string]string{
		"uri":              fmt.Sprintf("https://%v", routes[0].URL),
		"access_token_uri": fmt.Sprintf("%v/oauth/token", info.UAA()),
		"client_id":        clientId,
		"client_secret":    password,
	}
	broker.Logger.Info("broker.Bind: Binding Complete, returning credentials.")

	return binding, nil
}
