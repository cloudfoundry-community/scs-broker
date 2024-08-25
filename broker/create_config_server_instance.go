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

	appName := utilities.MakeAppName(serviceId, instanceId)

	service, err := broker.GetServiceByServiceID(serviceId)
	if err != nil {
		return "", err
	}
	broker.Logger.Info(fmt.Sprintf("CS %v => Service: %+v", appName, service))

	cfClient, err := broker.GetClient()
	if err != nil {
		return "", errors.New(fmt.Sprintf("CS %v => Couldn't Start CF Client Session: %s", appName, err.Error()))
	}

	spaceGUID := broker.Config.InstanceSpaceGUID
	buildpacks := []string{service.ServiceBuildpack}

	appConfig := resources.Application{
		Name:                appName,
		LifecycleType:       constant.AppLifecycleTypeBuildpack,
		LifecycleBuildpacks: buildpacks,
		StackName:           service.ServiceStack,
		State:               constant.ApplicationStopped,
		SpaceGUID:           spaceGUID,
	}
	broker.Logger.Info(fmt.Sprintf("CS %v => Config Server ccv3.Application Config: %+v", appName, appConfig))

	broker.Logger.Info(fmt.Sprintf("CS %v => Creating Config Server Application: %s", appName, appName))
	app, warn, err := cfClient.CreateApplication(appConfig)
	if err != nil {
		broker.Logger.Info(fmt.Sprintf("CS %v => ERROR from cfClient.CreateApplication(): %s", appName, err.Error()))
		return "", err
	}
	if warn != nil {
		broker.Logger.Info(fmt.Sprintf("WARN: %v", warn))
	}
	broker.Logger.Info(fmt.Sprintf("CS %v => App Created: %s as: %+v", appName, appName, app))

	info, _, _, err := cfClient.GetInfo()
	if err != nil {
		broker.Logger.Info(fmt.Sprintf("CS %v => ERROR from cfClient.GetInfo(): %s", appName, err.Error()))
		return "", err
	}
	broker.Logger.Info(fmt.Sprintf("CS %v => cf Client Info: %+v", appName, info))

	broker.Logger.Info(fmt.Sprintf("CS %v => Updating App Environment with jsonparams: %+v and params: %+v", appName, jsonparams, params))
	err = broker.UpdateAppEnvironment(cfClient, &app, &info, serviceId, instanceId, jsonparams, params)
	if err != nil {
		broker.Logger.Info(fmt.Sprintf("CS %v => ERROR from broker.UpdateAppEnvironment(): %s", appName, err.Error()))
		return "", err
	}

	if broker.Config.JavaConfig.JBPConfigOpenJDKJRE != "" {
		_, _, err = cfClient.UpdateApplicationEnvironmentVariables(app.GUID, ccv3.EnvironmentVariables{
			"JBP_CONFIG_OPEN_JDK_JRE": {Value: broker.Config.JavaConfig.JBPConfigOpenJDKJRE, IsSet: true},
		})
		if err != nil {
			broker.Logger.Info(fmt.Sprintf("CS %v => ERROR from cfClient.UpdateApplicationEnvironmentVariables(): %s", appName, err.Error()))
			return "", fmt.Errorf("CS %v => failed to set JBP_CONFIG_OPEN_JDK_JRE: %+v", appName, err)
		}
	}

	pkgConfig := ccv3.Package{
		Type: constant.PackageTypeBits,
		Relationships: resources.Relationships{
			constant.RelationshipTypeApplication: resources.Relationship{GUID: app.GUID},
		},
	}
	broker.Logger.Info(fmt.Sprintf("CS %v => Creating Package with config: %+v", appName, pkgConfig))
	pkg, _, err := cfClient.CreatePackage(pkgConfig)
	if err != nil {
		broker.Logger.Info(fmt.Sprintf("CS %v => ERROR from cfClient.CreatePackage(): %s", appName, err.Error()))
		return "", err
	}

	broker.Logger.Info(fmt.Sprintf("CS %v => Uploading Package: %+v", appName, pkg))

	jarname := path.Base(service.ServiceDownloadURI)
	artifact := broker.Config.ArtifactsDir + "/" + jarname
	broker.Logger.Info(fmt.Sprintf("CS %v => looking for artifact: %s", appName, artifact))
	fi, err := os.Stat(artifact)
	if err != nil {
		broker.Logger.Info(fmt.Sprintf("CS %v => ERROR from os.Stat(): %s", appName, err.Error()))
		return "", err
	}

	broker.Logger.Info(fmt.Sprintf("CS %v => Uploading: %s from %s size(%d)", appName, fi.Name(), artifact, fi.Size()))
	upkg, uwarnings, err := cfClient.UploadPackage(pkg, artifact)
	broker.showWarnings(uwarnings, upkg)
	if err != nil {
		broker.Logger.Info(fmt.Sprintf("CS %v => ERROR from cfClient.UploadPackage(): %s", appName, err.Error()))
		return "", err
	}

	broker.Logger.Info(fmt.Sprintf("CS %v => Polling Package", appName))
	pkg, pwarnings, err := broker.pollPackage(pkg)
	broker.showWarnings(pwarnings, pkg)
	if err != nil {
		broker.Logger.Info(fmt.Sprintf("CS %v => ERROR from broker.pollPackage(): %s", appName, err.Error()))
		return "", err
	}

	broker.Logger.Info(fmt.Sprintf("CS %v => Creating Build", appName))
	build, cwarnings, err := cfClient.CreateBuild(ccv3.Build{PackageGUID: pkg.GUID})
	broker.showWarnings(cwarnings, build)
	if err != nil {
		broker.Logger.Info(fmt.Sprintf("CS %v => ERROR from cfClient.CreateBuild(): %s", appName, err.Error()))
		return "", err
	}

	broker.Logger.Info(fmt.Sprintf("CS %v => Polling Build", appName))
	droplet, pbwarnings, err := broker.pollBuild(build.GUID, appName)
	broker.showWarnings(pbwarnings, droplet)
	if err != nil {
		broker.Logger.Info(fmt.Sprintf("CS %v => ERROR from broker.pollBuild(): %s", appName, err.Error()))
		return "", err
	}

	broker.Logger.Info(fmt.Sprintf("CS %v => Set application droplet", appName))
	_, _, err = cfClient.SetApplicationDroplet(app.GUID, droplet.GUID)
	if err != nil {
		broker.Logger.Info(fmt.Sprintf("CS %v => ERROR from cfClient.SetApplicationDroplet(): %s", appName, err.Error()))
		return "", err
	}
	domains, _, err := cfClient.GetDomains(
		ccv3.Query{Key: ccv3.NameFilter, Values: []string{broker.Config.InstanceDomain}},
	)
	if err != nil {
		broker.Logger.Info(fmt.Sprintf("CS %v => ERROR from cfClient.GetDomains(): %s", appName, err.Error()))
		return "", err
	}

	if len(domains) == 0 {
		msg := fmt.Sprintf("CS %v => no domains found for this instance", appName)
		broker.Logger.Info(msg)
		return "", errors.New(msg)
	}

	routeConfig := resources.Route{
		SpaceGUID:  spaceGUID,
		DomainGUID: domains[0].GUID,
		Host:       appName,
	}
	broker.Logger.Info(fmt.Sprintf("CS %v => Creating Route: %+v", appName, routeConfig))
	route, _, err := cfClient.CreateRoute(routeConfig)
	if err != nil {
		broker.Logger.Info(fmt.Sprintf("CS %v => ERROR from cfClient.CreateRoute(): %s", appName, err.Error()))
		return "", err
	}

	broker.Logger.Info(fmt.Sprintf("CS %v => Mapping Route", appName))
	_, err = cfClient.MapRoute(route.GUID, app.GUID)
	if err != nil {
		broker.Logger.Info(fmt.Sprintf("CS %v => ERROR from cfClient.MapRoute(): %s", appName, err.Error()))
		return "", err
	}

	broker.Logger.Info(fmt.Sprintf("CS %v => Updating Application: Restart", appName))
	app, _, err = cfClient.UpdateApplicationRestart(app.GUID)
	if err != nil {
		broker.Logger.Info(fmt.Sprintf("CS %v => ERROR from cfClient.UpdateApplicationRestart(): %s", appName, err.Error()))
		return "", err
	}

	broker.Logger.Info(fmt.Sprintf("CS %v => Route URL: %s", appName, route.URL))

	return route.URL, nil
}
