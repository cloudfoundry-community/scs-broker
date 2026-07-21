package broker

import (
	"context"
	"errors"

	brokerapi "code.cloudfoundry.org/brokerapi/v13/domain"
	"code.cloudfoundry.org/lager/v3"
	"github.com/cloudfoundry-community/scs-broker/broker/utilities"
	scsccparser "github.com/cloudfoundry-community/spring-cloud-services-cli-config-parser"
)

func (broker *SCSBroker) updateConfigServerInstance(cxt context.Context, instanceID string, details brokerapi.UpdateDetails, asyncAllowed bool) (brokerapi.UpdateServiceSpec, error) {
	spec := brokerapi.UpdateServiceSpec{}

	appName := utilities.MakeAppName(details.ServiceID, instanceID)
	spaceGUID := broker.Config.InstanceSpaceGUID

	broker.Logger.Info("broker.UpdateConfigServerInstance: update-service-instance", lager.Data{"plan-id": details.PlanID, "service-id": details.ServiceID})
	envsetup := scsccparser.EnvironmentSetup{}
	cfClient, err := broker.GetClient()
	if err != nil {
		broker.Logger.Error("broker.UpdateConfigServerInstance: broker.GetClient()", err)
		return spec, errors.New("Couldn't start session: " + err.Error())
	}

	info, _, err := cfClient.GetRoot()
	if err != nil {
		broker.Logger.Error("broker.UpdateConfigServerInstance: cfClient.GetRoot()", err)
		return spec, err
	}

	app, _, err := cfClient.GetApplicationByNameAndSpace(appName, spaceGUID)
	if err != nil {
		broker.Logger.Error("broker.UpdateConfigServerInstance: cfClient.GetApplicationByNameAndSpace()", err)
		return spec, errors.New("broker.UpdateConfigServerInstance: Couldn't find app session: " + err.Error())
	}

	mapparams, err := envsetup.ParseEnvironmentFromRaw(details.RawParameters)
	if err != nil {
		broker.Logger.Error("broker.UpdateConfigServerInstance: envsetup.ParseEnvironmentFromRaw()", err)
		return spec, err
	}

	broker.Logger.Info("broker.UpdateConfigServerInstance: Updating Environment")
	err = broker.UpdateAppEnvironment(cfClient, &app, &info, details.ServiceID, instanceID, string(details.RawParameters), mapparams)
	if err != nil {
		broker.Logger.Error("broker.UpdateConfigServerInstance: broker.UpdateAppEnvironment()", err)
		return spec, err
	}

	//TODO: Test this in particular, as it does not work as expected in
	//the equivalent workflow for service-registry.
	_, _, err = cfClient.UpdateApplication(utilities.SafeApp(app))
	if err != nil {
		broker.Logger.Error("broker.UpdateConfigServerInstance: cfClient.UpdateApplication()", err)
		return spec, err
	}

	_, _, err = cfClient.UpdateApplicationRestart(app.GUID)
	if err != nil {
		broker.Logger.Error("broker.updateConfigServerInstance: cfClient.UpdateApplicationRestart()", err)
		return spec, err
	}

	return spec, nil
}
