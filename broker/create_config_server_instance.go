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
		broker.Logger.Error("CCS %v: broker.GetServiceByServiceID()", err)
		return "", err
	}
	broker.Logger.Info(fmt.Sprintf("CCS %v => Service: %+v", appName, service))

	cfClient, err := broker.GetClient()
	if err != nil {
		msg := fmt.Sprintf("CCS %v: broker.GetClient() Couldn't start CF Client Session.", appName)
		broker.Logger.Error(msg, err)
		return "", err
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
	broker.Logger.Info(fmt.Sprintf("CCS %v => Config Server ccv3.Application Config: %+v", appName, appConfig))

	broker.Logger.Info(fmt.Sprintf("CCS %v => Creating Config Server Application: %s", appName, appName))
	app, warn, err := cfClient.CreateApplication(appConfig)
	if err != nil {
		broker.Logger.Error(fmt.Sprintf("CCS %v => ERROR from cfClient.CreateApplication()", appName), err)
		return "", err
	}
	if warn != nil {
		broker.Logger.Info(fmt.Sprintf("WARN: %v", warn))
	}
	broker.Logger.Info(fmt.Sprintf("CCS %v => App Created as: %+v", appName, app))

	info, _, _, err := cfClient.GetInfo()
	if err != nil {
		broker.Logger.Error(fmt.Sprintf("CCS %v => ERROR from cfClient.GetInfo()", appName), err)
		return "", err
	}
	broker.Logger.Info(fmt.Sprintf("CCS %v => cf Client Info: %+v", appName, info))

	broker.Logger.Info(fmt.Sprintf("CCS %v => Updating App Environment with jsonparams: %+v and params: %+v", appName, jsonparams, params))
	err = broker.UpdateAppEnvironment(cfClient, &app, &info, serviceId, instanceId, jsonparams, params)
	if err != nil {
		broker.Logger.Error(fmt.Sprintf("CCS %v => ERROR from broker.UpdateAppEnvironment()", appName), err)
		return "", err
	}

	if broker.Config.JavaConfig.JBPConfigOpenJDKJRE != "" {
		_, _, err = cfClient.UpdateApplicationEnvironmentVariables(app.GUID, ccv3.EnvironmentVariables{
			"JBP_CONFIG_OPEN_JDK_JRE": {Value: broker.Config.JavaConfig.JBPConfigOpenJDKJRE, IsSet: true},
		})
		if err != nil {
			broker.Logger.Error(fmt.Sprintf("CCS %v => ERROR from cfClient.UpdateApplicationEnvironmentVariables(), failed to set JBP_CONFIG_OPEN_JDK_JRE", appName), err)
			return "", err
		}
	}

	pkgConfig := ccv3.Package{
		Type: constant.PackageTypeBits,
		Relationships: resources.Relationships{
			constant.RelationshipTypeApplication: resources.Relationship{GUID: app.GUID},
		},
	}
	broker.Logger.Info(fmt.Sprintf("CCS %v => Creating Package with config: %+v", appName, pkgConfig))
	pkg, _, err := cfClient.CreatePackage(pkgConfig)
	if err != nil {
		broker.Logger.Error(fmt.Sprintf("CCS %v => ERROR from cfClient.CreatePackage()", appName), err)
		return "", err
	}

	broker.Logger.Info(fmt.Sprintf("CCS %v => Uploading Package: %+v", appName, pkg))

	jarname := path.Base(service.ServiceDownloadURI)
	artifact := broker.Config.ArtifactsDir + "/" + jarname
	broker.Logger.Info(fmt.Sprintf("CCS %v => looking for artifact: %s", appName, artifact))
	fi, err := os.Stat(artifact)
	if err != nil {
		broker.Logger.Error(fmt.Sprintf("CCS %v => ERROR from os.Stat()", appName), err)
		return "", err
	}

	broker.Logger.Info(fmt.Sprintf("CCS %v => Uploading: %s from %s size(%d)", appName, fi.Name(), artifact, fi.Size()))
	upkg, uwarnings, err := cfClient.UploadPackage(pkg, artifact)
	broker.showWarnings(uwarnings, upkg)
	if err != nil {
		broker.Logger.Error(fmt.Sprintf("CCS %v => ERROR from cfClient.UploadPackage()", appName), err)
		return "", err
	}

	broker.Logger.Info(fmt.Sprintf("CCS %v => Polling Package", appName))
	pkg, pwarnings, err := broker.pollPackage(pkg)
	broker.showWarnings(pwarnings, pkg)
	if err != nil {
		broker.Logger.Error(fmt.Sprintf("CCS %v => ERROR from broker.pollPackage()", appName), err)
		return "", err
	}

	broker.Logger.Info(fmt.Sprintf("CCS %v => Creating Build", appName))
	build, cwarnings, err := cfClient.CreateBuild(ccv3.Build{PackageGUID: pkg.GUID})
	broker.showWarnings(cwarnings, build)
	if err != nil {
		broker.Logger.Error(fmt.Sprintf("CCS %v => ERROR from cfClient.CreateBuild()", appName), err)
		return "", err
	}

	broker.Logger.Info(fmt.Sprintf("CCS %v => Polling Build", appName))
	droplet, pbwarnings, err := broker.pollBuild(build.GUID, appName)
	broker.showWarnings(pbwarnings, droplet)
	if err != nil {
		broker.Logger.Error(fmt.Sprintf("CCS %v => ERROR from broker.pollBuild()", appName), err)
		return "", err
	}

	broker.Logger.Info(fmt.Sprintf("CCS %v => Set application droplet", appName))
	_, _, err = cfClient.SetApplicationDroplet(app.GUID, droplet.GUID)
	if err != nil {
		broker.Logger.Error(fmt.Sprintf("CCS %v => ERROR from cfClient.SetApplicationDroplet()", appName), err)
		return "", err
	}
	domains, _, err := cfClient.GetDomains(
		ccv3.Query{Key: ccv3.NameFilter, Values: []string{broker.Config.InstanceDomain}},
	)
	if err != nil {
		broker.Logger.Error(fmt.Sprintf("CCS %v => ERROR from cfClient.GetDomains()", appName), err)
		return "", err
	}

	if len(domains) == 0 {
		msg := fmt.Sprintf("CCS %v => no domains found for this instance", appName)
		broker.Logger.Info(msg)
		return "", errors.New(msg)
	}

	routeConfig := resources.Route{
		SpaceGUID:  spaceGUID,
		DomainGUID: domains[0].GUID,
		Host:       appName,
	}
	broker.Logger.Info(fmt.Sprintf("CCS %v => Creating Route: %+v", appName, routeConfig))
	route, _, err := cfClient.CreateRoute(routeConfig)
	if err != nil {
		broker.Logger.Error(fmt.Sprintf("CCS %v => ERROR from cfClient.CreateRoute()", appName), err)
		return "", err
	}

	broker.Logger.Info(fmt.Sprintf("CCS %v => Mapping Route", appName))
	_, err = cfClient.MapRoute(route.GUID, app.GUID)
	if err != nil {
		broker.Logger.Error(fmt.Sprintf("CCS %v => ERROR from cfClient.MapRoute()", appName), err)
		return "", err
	}

	broker.Logger.Info(fmt.Sprintf("CCS %v => Updating Application: Restart", appName))
	app, _, err = cfClient.UpdateApplicationRestart(app.GUID)
	if err != nil {
		broker.Logger.Error(fmt.Sprintf("CCS %v => ERROR from cfClient.UpdateApplicationRestart()", appName), err)
		return "", err
	}

	broker.Logger.Info(fmt.Sprintf("CCS %v => Route URL: %s", appName, route.URL))

	return route.URL, nil
}
