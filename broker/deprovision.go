package broker

import (
	"context"

	"code.cloudfoundry.org/cli/api/cloudcontroller/ccerror"
	"github.com/cloudfoundry-community/scs-broker/broker/utilities"
	brokerapi "github.com/pivotal-cf/brokerapi/domain"
)

func (broker *SCSBroker) Deprovision(ctx context.Context, instanceID string, details brokerapi.DeprovisionDetails, asyncAllowed bool) (brokerapi.DeprovisionServiceSpec, error) {
	spec := brokerapi.DeprovisionServiceSpec{}

	cfClient, err := broker.GetClient()
	if err != nil {
		broker.Logger.Error("broker.Deprovision: broker.GetClient()", err)
		return spec, err
	}
	appName := utilities.MakeAppName(details.ServiceID, instanceID)
	app, _, err := cfClient.GetApplicationByNameAndSpace(appName, broker.Config.InstanceSpaceGUID)
	appNotFound := ccerror.ApplicationNotFoundError{Name: appName}
	if err == appNotFound {
		broker.Logger.Error("broker.Deprovision: broker.GetApplicationByNameAndSpace(): Application Not Found!", err)
		return spec, nil
	} else if err != nil {
		broker.Logger.Error("broker.Deprovision: broker.GetApplicationByNameAndSpace()", err)
		return spec, err
	}
	routes, _, err := cfClient.GetApplicationRoutes(app.GUID)
	if err != nil {
		broker.Logger.Error("broker.Deprovision: broker.GetApplicationRoutes()", err)
		return spec, err
	}
	_, _, err = cfClient.UpdateApplicationStop(app.GUID)
	if err != nil {
		broker.Logger.Error("broker.Deprovision: broker.UpdateApplicationStop()", err)
		return spec, err
	}

	for route := range routes {
		_, _, err := cfClient.DeleteRoute(routes[route].GUID)
		broker.Logger.Error("broker.Deprovision: broker.DeleteRoute()", err)
		if err != nil {
			return spec, err
		}
	}

	_, _, err = cfClient.DeleteApplication(app.GUID)
	if err != nil {
		broker.Logger.Error("broker.Deprovision: broker.DeleteApplication()", err)
		return spec, err
	}

	return spec, nil
}
