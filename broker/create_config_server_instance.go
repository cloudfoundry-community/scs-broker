package broker

import (
	"errors"
	"fmt"
	"os"
	"path"

	"code.cloudfoundry.org/cli/api/cloudcontroller/ccv3"
	"code.cloudfoundry.org/cli/api/cloudcontroller/ccv3/constant"
	"code.cloudfoundry.org/cli/resources"
	"github.com/cloudfoundry-community/scs-broker/broker/utilities"
)

func (broker *SCSBroker) createConfigServerInstance(serviceId string, instanceId string, jsonparams string, params map[string]string) (string, error) {

	service, err := broker.GetServiceByServiceID(serviceId)
	if err != nil {
		return "", err
	}
	broker.Logger.Info(fmt.Sprintf("CS %s => Service: %v", instanceId, service))

	cfClient, err := broker.GetClient()
	if err != nil {
		return "", errors.New(fmt.Sprintf("CS %s => Couldn't Start CF Client Session: %s", instanceId, err.Error()))
	}

	appName := utilities.MakeAppName(serviceId, instanceId)
	spaceGUID := broker.Config.InstanceSpaceGUID
	buildpacks := []string{service.ServiceBuildpack}

	appConfig := resources.Application{
		Name:                appName,
		LifecycleType:       constant.AppLifecycleTypeBuildpack,
		LifecycleBuildpacks: buildpacks,
		State:               constant.ApplicationStopped,
		SpaceGUID:           spaceGUID,
	}
	broker.Logger.Info(fmt.Sprintf("CS %s => Config Server ccv3.Application Config: %v", instanceId, appConfig))

	broker.Logger.Info(fmt.Sprintf("CS %s => Creating Config Server Application: %s", instanceId, appName))
	app, warn, err := cfClient.CreateApplication(appConfig)
	if err != nil {
		return "", err
	}
	if warn != nil {
		broker.Logger.Info(fmt.Sprintf("WARN: %s", warn))
	}
	broker.Logger.Info(fmt.Sprintf("CS %s => App Created: %s as: %+v", instanceId, appName, app))

	info, _, _, err := cfClient.GetInfo()
	if err != nil {
		return "", err
	}
	broker.Logger.Info(fmt.Sprintf("CS %s => cf Client Info: %+v", instanceId, info))

	broker.Logger.Info(fmt.Sprintf("CS %s => Updating App Environment with jsonparams: %v and params: %v", instanceId, jsonparams, params))
	err = broker.UpdateAppEnvironment(cfClient, &app, &info, serviceId, instanceId, jsonparams, params)
	if err != nil {
		return "", err
	}

	if broker.Config.JavaConfig.JBPConfigOpenJDKJRE != "" {
		//_, _, err = cfClient.UpdateApplicationEnvironmentVariables(app.GUID, resources.EnvironmentVariables{
		//	"JBP_CONFIG_OPEN_JDK_JRE": {Value: broker.Config.JavaConfig.JBPConfigOpenJDKJRE, IsSet: true},
		//})
		_, _, err = cfClient.UpdateApplicationEnvironmentVariables(app.GUID, ccv3.EnvironmentVariables{
			"JBP_CONFIG_OPEN_JDK_JRE": {Value: broker.Config.JavaConfig.JBPConfigOpenJDKJRE, IsSet: true},
		})
		if err != nil {
			return "", fmt.Errorf("CS %s => failed to set JBP_CONFIG_OPEN_JDK_JRE: %v", instanceId, err)
		}
	}

	pkgConfig := ccv3.Package{
		Type: constant.PackageTypeBits,
		Relationships: resources.Relationships{
			constant.RelationshipTypeApplication: resources.Relationship{GUID: app.GUID},
		},
	}
	broker.Logger.Info(fmt.Sprintf("CS %s => Creating Package with config: %v", instanceId, pkgConfig))
	pkg, _, err := cfClient.CreatePackage(pkgConfig)
	if err != nil {
		return "", err
	}

	broker.Logger.Info(fmt.Sprintf("CS %s => Uploading Package: %v", instanceId, pkg))

	jarname := path.Base(service.ServiceDownloadURI)
	artifact := broker.Config.ArtifactsDir + "/" + jarname
	broker.Logger.Info(fmt.Sprintf("CS %s => looking for artifact: %s", instanceId, artifact))
	fi, err := os.Stat(artifact)
	if err != nil {
		return "", err
	}

	broker.Logger.Info(fmt.Sprintf("CS %s => Uploading: %s from %s size(%d)", instanceId, fi.Name(), artifact, fi.Size()))
	upkg, uwarnings, err := cfClient.UploadPackage(pkg, artifact)
	broker.showWarnings(uwarnings, upkg)
	if err != nil {
		return "", err
	}

	broker.Logger.Info(fmt.Sprintf("CS %s => Polling Package", instanceId))
	pkg, pwarnings, err := broker.pollPackage(pkg)
	broker.showWarnings(pwarnings, pkg)
	if err != nil {

		return "", err
	}

	broker.Logger.Info(fmt.Sprintf("CS %s => Creating Build", instanceId))
	build, cwarnings, err := cfClient.CreateBuild(ccv3.Build{PackageGUID: pkg.GUID})
	broker.showWarnings(cwarnings, build)
	if err != nil {
		return "", err
	}

	broker.Logger.Info(fmt.Sprintf("CS %s => Polling Build", instanceId))
	droplet, pbwarnings, err := broker.pollBuild(build.GUID, appName)
	broker.showWarnings(pbwarnings, droplet)
	if err != nil {
		return "", err
	}

	broker.Logger.Info(fmt.Sprintf("CS %s => Set application droplet", instanceId))
	_, _, err = cfClient.SetApplicationDroplet(app.GUID, droplet.GUID)
	if err != nil {
		return "", err
	}
	domains, _, err := cfClient.GetDomains(
		ccv3.Query{Key: ccv3.NameFilter, Values: []string{broker.Config.InstanceDomain}},
	)
	if err != nil {
		return "", err
	}

	if len(domains) == 0 {
		return "", errors.New(fmt.Sprintf("CS %s => no domains found for this instance", instanceId))
	}

	routeConfig := resources.Route{
		SpaceGUID:  spaceGUID,
		DomainGUID: domains[0].GUID,
		Host:       appName,
	}
	broker.Logger.Info(fmt.Sprintf("CS %s => Creating Route: %v", instanceId, routeConfig))
	route, _, err := cfClient.CreateRoute(routeConfig)
	if err != nil {
		return "", err
	}
	broker.Logger.Info(fmt.Sprintf("CS %s => Mapping Route", instanceId))
	_, err = cfClient.MapRoute(route.GUID, app.GUID)
	if err != nil {
		return "", err
	}
	broker.Logger.Info(fmt.Sprintf("CS %s => Updating Application: Restart", instanceId))
	app, _, err = cfClient.UpdateApplicationRestart(app.GUID)
	if err != nil {
		return "", err
	}

	broker.Logger.Info(fmt.Sprintf("CS %s => Route URL: %s", instanceId, route.URL))

	return route.URL, nil
}
