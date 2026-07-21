package broker

import (
	"context"
	"errors"
	"fmt"

	brokerapi "code.cloudfoundry.org/brokerapi/v13/domain"
	"code.cloudfoundry.org/cli/v8/resources"
	"github.com/cloudfoundry-community/go-uaa"
	"github.com/cloudfoundry-community/scs-broker/broker/utilities"
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
	info, _, err := cfClient.GetRoot()
	if err != nil {
		broker.Logger.Error("broker.Bind cfClient.GetRoot()", err)
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
	creds, err := buildBindingCredentials(routes, info.UAA(), clientId, password)
	if err != nil {
		broker.Logger.Error("broker.Bind buildBindingCredentials()", err)
		return binding, err
	}
	binding.Credentials = creds
	broker.Logger.Info("broker.Bind: Binding Complete, returning credentials.")

	return binding, nil
}

// buildBindingCredentials assembles the OSB binding credentials from the
// backing app's routes. A backing app that failed to stage has no route, so the
// route slice is checked before indexing: without this guard Bind panics and
// the CC sees a 502 instead of a usable error (plan decision D4).
func buildBindingCredentials(routes []resources.Route, uaaURL, clientID, password string) (map[string]string, error) {
	if len(routes) == 0 {
		return nil, errors.New("service instance backing app has no route; it may have failed to stage")
	}

	return map[string]string{
		"uri":              fmt.Sprintf("https://%v", routes[0].URL),
		"access_token_uri": fmt.Sprintf("%v/oauth/token", uaaURL),
		"client_id":        clientID,
		"client_secret":    password,
	}, nil
}
