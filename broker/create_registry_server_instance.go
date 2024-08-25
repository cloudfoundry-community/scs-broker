package broker

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"strconv"
	"time"

	"code.cloudfoundry.org/cli/api/cloudcontroller/ccv3"
	"code.cloudfoundry.org/cli/api/cloudcontroller/ccv3/constant"
	"code.cloudfoundry.org/cli/resources"
	"github.com/cloudfoundry-community/scs-broker/broker/utilities"
)

// jsonparams are the parameters passed in via the -c '{}' cf cli command line argument when creating the service instance.
func (broker *SCSBroker) createRegistryServerInstance(serviceId string, instanceId string, jsonparams string, params map[string]string) (string, error) {

	appName := utilities.MakeAppName(serviceId, instanceId)

	service, err := broker.GetServiceByServiceID(serviceId)
	if err != nil {
		broker.Logger.Error("CRS %v: broker.GetServiceByServiceID()", err)
		return "", err
	}
	broker.Logger.Info(fmt.Sprintf("CRS %v => Service: %v", appName, service))

	rc := utilities.NewRegistryConfig()
	rp, err := utilities.ExtractRegistryParams(jsonparams)
	if err != nil {
		broker.Logger.Error("CRS %v: utilities.NewRegistryConfig()", err)
		return "", err
	}
	broker.Logger.Info(fmt.Sprintf("CRS %v => Params: %v", appName, rp))

	count, err := rp.Count()
	if err != nil {
		broker.Logger.Error("CRS %v: rp.Count()", err)
		return "", err
	}
	broker.Logger.Info(fmt.Sprintf("CRS %v => count: %v", appName, count))

	cfClient, err := broker.GetClient()
	if err != nil {
		broker.Logger.Error("CRS %v: broker.GetClient(): Couldn't Start CF Client Session", err)
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
	broker.Logger.Info(fmt.Sprintf("CRS %v => resources.Application Config: %v", appName, appConfig))

	broker.Logger.Info(fmt.Sprintf("CRS %v => Creating Application: %s", appName, appName))
	app, warn, err := cfClient.CreateApplication(appConfig)
	if err != nil {
		broker.Logger.Error(fmt.Sprintf("CRS %v => ERROR from cfClient.CreateApplication()", appName), err)
		return "", err
	}
	broker.Logger.Info(fmt.Sprintf("CRS %v => Application Created: %s as: %+v", appName, appName, app))

	info, _, _, err := cfClient.GetInfo()
	if err != nil {
		broker.Logger.Error(fmt.Sprintf("CRS %v => ERROR from cfClient.GetInfo()", appName), err)
		return "", err
	}
	if warn != nil {
		broker.Logger.Info(fmt.Sprintf("WARN: %v", warn))
	}
	broker.Logger.Info(fmt.Sprintf("CRS %v => App Created: %s as: %+v", appName, appName, app))

	broker.Logger.Info(fmt.Sprintf("CRS %v => Updating App Environment with jsonparams: %v and params: %v", appName, jsonparams, params))
	err = broker.UpdateAppEnvironment(cfClient, &app, &info, serviceId, instanceId, jsonparams, params)
	if err != nil {
		broker.Logger.Error(fmt.Sprintf("CRS %v => ERROR from broker.UpdateAppEnvironment()", appName), err)
		return "", err
	}

	if broker.Config.JavaConfig.JBPConfigOpenJDKJRE != "" {
		_, _, err = cfClient.UpdateApplicationEnvironmentVariables(app.GUID, ccv3.EnvironmentVariables{
			"JBP_CONFIG_OPEN_JDK_JRE": {Value: broker.Config.JavaConfig.JBPConfigOpenJDKJRE, IsSet: true},
		})
		if err != nil {
			broker.Logger.Error(fmt.Sprintf("RCS %v => ERROR from cfClient.UpdateApplicationEnvironmentVariables(), failed to set JBP_CONFIG_OPEN_JDK_JRE", appName), err)
			return "", err
		}
	}

	pkgConfig := ccv3.Package{
		Type: constant.PackageTypeBits,
		Relationships: resources.Relationships{
			constant.RelationshipTypeApplication: resources.Relationship{GUID: app.GUID},
		},
	}
	broker.Logger.Info("CRS %v => Creating Package")
	pkg, _, err := cfClient.CreatePackage(pkgConfig)
	if err != nil {
		broker.Logger.Error(fmt.Sprintf("CRS %v => ERROR from cfClient.CreatePackage()", appName), err)
		return "", err
	}

	broker.Logger.Info(fmt.Sprintf("CRS %v => Uploading Package", appName))

	jarname := path.Base(service.ServiceDownloadURI)
	artifact := broker.Config.ArtifactsDir + "/" + jarname

	fi, err := os.Stat(artifact)
	if err != nil {
		broker.Logger.Error(fmt.Sprintf("CRS %v => ERROR from os.Stat(artifact)", appName), err)
		return "", err
	}

	broker.Logger.Info(fmt.Sprintf("CRS %v => Uploading: %s from %s size(%d)", appName, fi.Name(), artifact, fi.Size()))
	upkg, uwarnings, err := cfClient.UploadPackage(pkg, artifact)
	broker.showWarnings(uwarnings, upkg)
	if err != nil {
		broker.Logger.Error(fmt.Sprintf("CRS %v => ERROR from cfClient.UploadPackage(pkg,artifact)", appName), err)
		return "", err
	}

	broker.Logger.Info(fmt.Sprintf("CRS %v => Polling Package", appName))
	pkg, pwarnings, err := broker.pollPackage(pkg)
	broker.showWarnings(pwarnings, pkg)
	if err != nil {
		broker.Logger.Error(fmt.Sprintf("CRS %v => ERROR from broker.pollPackage(pkg)", appName), err)
		return "", err
	}

	broker.Logger.Info(fmt.Sprintf("CRS %v => Creating Build", appName))
	build, cwarnings, err := cfClient.CreateBuild(ccv3.Build{PackageGUID: pkg.GUID})
	broker.showWarnings(cwarnings, build)
	if err != nil {
		broker.Logger.Error(fmt.Sprintf("CRS %v => ERROR from broker.CreateBuild()", appName), err)
		return "", err
	}

	broker.Logger.Info(fmt.Sprintf("CRS %v => polling build", appName))
	droplet, pbwarnings, err := broker.pollBuild(build.GUID, appName)
	broker.showWarnings(pbwarnings, droplet)
	if err != nil {
		broker.Logger.Error(fmt.Sprintf("CRS %v => ERROR from broker.pollBuild()", appName), err)
		return "", err
	}

	broker.Logger.Info(fmt.Sprintf("CRS %v => set application droplet", appName))
	_, _, err = cfClient.SetApplicationDroplet(app.GUID, droplet.GUID)
	if err != nil {
		broker.Logger.Error(fmt.Sprintf("CRS %v => ERROR from cfClient.SetApplicationDroplet(app.GUID, droplet.GUID)", appName), err)
		return "", err
	}
	domains, _, err := cfClient.GetDomains(
		ccv3.Query{Key: ccv3.NameFilter, Values: []string{broker.Config.InstanceDomain}},
	)
	if err != nil {
		broker.Logger.Error(fmt.Sprintf("CRS %v => ERROR from cfClient.GetDomains()", appName), err)
		return "", err
	}

	if len(domains) == 0 {
		msg := fmt.Sprintf("CRS %v => no domains found for this instance", appName)
		broker.Logger.Info(msg)
		return "", errors.New(msg)
	}

	routeConfig := resources.Route{
		SpaceGUID:  spaceGUID,
		DomainGUID: domains[0].GUID,
		Host:       appName,
	}
	broker.Logger.Info(fmt.Sprintf("CRS %v => Creating Route %+v", appName, routeConfig))
	route, _, err := cfClient.CreateRoute(routeConfig)
	if err != nil {
		broker.Logger.Error(fmt.Sprintf("CRS %v => ERROR from cfClient.CreateRoute(%+v)", appName, routeConfig), err)
		return "", err
	}

	broker.Logger.Info(fmt.Sprintf("CRS %v => Mapping Route cfClient.MapRoute(route.GUID,app.GUI)", appName))
	_, err = cfClient.MapRoute(route.GUID, app.GUID)
	if err != nil {
		broker.Logger.Error(fmt.Sprintf("CRS %v => ERROR from cfClient.MapRoute(%v,%v)", appName, route.GUID, app.GUID), err)
		return "", err
	}

	time.Sleep(time.Second)

	broker.Logger.Info(fmt.Sprintf("CRS %v => Starting Application", appName))
	app, _, err = cfClient.UpdateApplicationStart(app.GUID)
	if err != nil {
		broker.Logger.Error(fmt.Sprintf("CRS %v => Application Start Failed, Trying restart", appName), err)
		app, _, err = cfClient.UpdateApplicationRestart(app.GUID)
		if err != nil {
			broker.Logger.Error(fmt.Sprintf("CRS %v => Application Start failed", appName), err)
			return "", err
		}
	}

	broker.Logger.Info(fmt.Sprintf("CRS %v => handling node count", appName))
	// handle the node count
	if count > 1 {
		rc.Clustered()
		broker.Logger.Info(fmt.Sprintf("CRS %v => scaling to %d", appName, count))
		err = broker.scaleRegistryServer(cfClient, &app, count)
		if err != nil {
			broker.Logger.Error(fmt.Sprintf("CRS %v => ERROR from broker.scaleRegistryServer()", appName), err)
			return "", err
		}

		community, err := broker.GetCommunity()
		if err != nil {
			broker.Logger.Error(fmt.Sprintf("CRS %v => ERROR from broker.GetCommunity()", appName), err)
			return "", err
		}

		stats, err := getProcessStatsByAppAndType(cfClient, community, broker.Logger, app.GUID, "web")
		if err != nil {
			broker.Logger.Error(fmt.Sprintf("CRS %v => ERROR from getProcessStatsByAppAndType()", appName), err)
			return "", nil
		}

		for _, stat := range stats {
			rc.AddPeer(stat.Index, fmt.Sprintf("http://%s:%d/eureka", stat.Host, stat.InstancePorts[0].External), serviceId)
		}
	} else {
		rc.Standalone()
	}

	broker.Logger.Info(fmt.Sprintf("CRS %v => Updating Environment", appName))
	err = broker.UpdateRegistryEnvironment(cfClient, &app, &info, serviceId, instanceId, rc, params)
	if err != nil {
		broker.Logger.Error(fmt.Sprintf("CRS %v => ERROR from broker.UpdateRegistryEnvironment()", appName), err)
		return "", err
	}

	broker.Logger.Info(fmt.Sprintf("CRS %v => Starting Application", appName))
	app, _, err = cfClient.UpdateApplicationStart(app.GUID)
	if err != nil {
		broker.Logger.Error(fmt.Sprintf("CRS %v => ERROR from cfClient.UpdateApplicationStart() ; Attempting Restart.", appName), err)
		app, _, err = cfClient.UpdateApplicationRestart(app.GUID)
		if err != nil {
			broker.Logger.Error(fmt.Sprintf("CRS %v => ERROR from cfClient.UpdateApplicationRestart()", appName), err)
			return "", err
		}
	}

	community, err := broker.GetCommunity()
	if err != nil {
		broker.Logger.Error(fmt.Sprintf("CRS %v => ERROR from broker.GetCommunity()", appName), err)
		return "", err
	}

	if count > 1 {
		stats, err := getProcessStatsByAppAndType(cfClient, community, broker.Logger, app.GUID, "web")
		if err != nil {
			broker.Logger.Error(fmt.Sprintf("CRS %v => ERROR from getProcessStatsByAppAndType()", appName), err)
			return "", err
		}

		for _, stat := range stats {
			rc.AddPeer(stat.Index, fmt.Sprintf("http://%s:%d/eureka", stat.Host, stat.InstancePorts[0].External), serviceId)
		}
	}

	peers, err := json.Marshal(rc.Peers)
	if err != nil {
		broker.Logger.Error(fmt.Sprintf("CRS %v => ERROR from json.Marshal()", appName), err)
		return "", err
	}
	x := 0
	for _, peer := range rc.Peers {
		req, err := http.NewRequest(http.MethodPost, "https://"+route.URL+"/config/peers", bytes.NewBuffer(peers))
		if err != nil {
			broker.Logger.Error(fmt.Sprintf("CRS %v => ERROR from http.NewRequest()", appName), err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Cf-App-Instance", app.GUID+":"+strconv.Itoa(peer.Index))

		refreshreq, err := http.NewRequest(http.MethodPost, "https://"+route.URL+"/actuator/refresh", nil)
		if err != nil {
			broker.Logger.Error(fmt.Sprintf("CRS %v => ERROR from http.NewRequest()", appName), err)
		}
		refreshreq.Header.Set("Content-Type", "application/json")
		refreshreq.Header.Set("X-Cf-App-Instance", app.GUID+":"+strconv.Itoa(peer.Index))

		client := http.Client{
			Timeout: 30 * time.Second,
		}

		res, err := client.Do(req)
		if err != nil {
			broker.Logger.Error(fmt.Sprintf("CRS %v => ERROR making http request from client.Do()", appName), err)
		}
		broker.Logger.Info(res.Request.RequestURI)
		broker.Logger.Info(string(peers))
		broker.Logger.Info(res.Status)

		refreshres, err := client.Do(refreshreq)
		if err != nil {
			broker.Logger.Error(fmt.Sprintf("CRS %v => ERROR making http request from client.Do()", appName), err)
		}
		broker.Logger.Info(refreshres.Request.RequestURI)
		broker.Logger.Info(string(peers))
		broker.Logger.Info(refreshres.Status)
		x++
	}

	broker.Logger.Info(route.URL)

	successfulStart, err := broker.MonitorApplicationStartup(cfClient, community, app.GUID)
	if err != nil || !successfulStart {
		broker.Logger.Error(fmt.Sprintf("CRS %v => Crashed application restarting...", appName), err)
		app, _, err = cfClient.UpdateApplicationStart(app.GUID)
		if err != nil {
			broker.Logger.Error(fmt.Sprintf("CRS %v => Application Start Failed, Trying restart...", appName), err)
			app, _, err = cfClient.UpdateApplicationRestart(app.GUID)
			if err != nil {
				broker.Logger.Error(fmt.Sprintf("CRS %v => Application Start failed.", appName), err)
				return "", err
			}
		}
	}

	return route.URL, nil
}
