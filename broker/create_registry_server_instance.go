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

	service, err := broker.GetServiceByServiceID(serviceId)
	if err != nil {
		return "", err
	}
	broker.Logger.Info(fmt.Sprintf("RS %v => Service: %v", instanceId, service))

	rc := utilities.NewRegistryConfig()
	rp, err := utilities.ExtractRegistryParams(jsonparams)
	if err != nil {
		return "", err
	}
	broker.Logger.Info(fmt.Sprintf("RS %v => Params: %v", instanceId, rp))

	count, err := rp.Count()
	if err != nil {
		return "", err
	}
	broker.Logger.Info(fmt.Sprintf("RS %v => count: %v", instanceId, count))

	cfClient, err := broker.GetClient()
	if err != nil {
		return "", errors.New(fmt.Sprintf("RS %v => Couldn't Start CF Client Session: %s", instanceId, err.Error()))
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
	broker.Logger.Info(fmt.Sprintf("RS %v => resources.Application Config: %v", instanceId, appConfig))

	broker.Logger.Info(fmt.Sprintf("RS %v => Creating Application: %s", instanceId, appName))
	app, warn, err := cfClient.CreateApplication(appConfig)
	if err != nil {
		broker.Logger.Info(fmt.Sprintf("RS %v => ERROR from cfClient.CreateApplication(): %s", instanceId, err.Error()))
		return "", err
	}
	broker.Logger.Info(fmt.Sprintf("RS %v => Application Created: %s as: %+v", instanceId, appName, app))

	info, _, _, err := cfClient.GetInfo()
	if err != nil {
		broker.Logger.Info(fmt.Sprintf("RS %v => ERROR from cfClient.GetInfo(): %s", instanceId, err.Error()))
		return "", err
	}
	if warn != nil {
		broker.Logger.Info(fmt.Sprintf("WARN: %v", warn))
	}
	broker.Logger.Info(fmt.Sprintf("RS %v => App Created: %s as: %+v", instanceId, appName, app))

	broker.Logger.Info(fmt.Sprintf("RS %v => Updating App Environment with jsonparams: %v and params: %v", instanceId, jsonparams, params))
	err = broker.UpdateAppEnvironment(cfClient, &app, &info, serviceId, instanceId, jsonparams, params)
	if err != nil {
		broker.Logger.Info(fmt.Sprintf("RS %v => ERROR from broker.UpdateAppEnvironment(): %s", instanceId, err.Error()))
		return "", err
	}

	if broker.Config.JavaConfig.JBPConfigOpenJDKJRE != "" {
		_, _, err = cfClient.UpdateApplicationEnvironmentVariables(app.GUID, ccv3.EnvironmentVariables{
			"JBP_CONFIG_OPEN_JDK_JRE": {Value: broker.Config.JavaConfig.JBPConfigOpenJDKJRE, IsSet: true},
		})
	}

	broker.Logger.Info("RS %v => Creating Package")
	pkg, _, err := cfClient.CreatePackage(
		ccv3.Package{
			Type: constant.PackageTypeBits,
			Relationships: resources.Relationships{
				constant.RelationshipTypeApplication: resources.Relationship{GUID: app.GUID},
			},
		})
	if err != nil {
		return "", err
	}

	broker.Logger.Info(fmt.Sprintf("RS %v => Uploading Package", instanceId))

	jarname := path.Base(service.ServiceDownloadURI)
	artifact := broker.Config.ArtifactsDir + "/" + jarname

	fi, err := os.Stat(artifact)
	if err != nil {
		return "", err
	}

	broker.Logger.Info(fmt.Sprintf("RS %v => Uploading: %s from %s size(%d)", instanceId, fi.Name(), artifact, fi.Size()))

	upkg, uwarnings, err := cfClient.UploadPackage(pkg, artifact)
	broker.showWarnings(uwarnings, upkg)
	if err != nil {
		return "", err
	}

	broker.Logger.Info(fmt.Sprintf("RS %v => Polling Package", instanceId))
	pkg, pwarnings, err := broker.pollPackage(pkg)
	broker.showWarnings(pwarnings, pkg)
	if err != nil {

		return "", err
	}

	broker.Logger.Info(fmt.Sprintf("RS %v => Creating Build", instanceId))
	build, cwarnings, err := cfClient.CreateBuild(ccv3.Build{PackageGUID: pkg.GUID})
	broker.showWarnings(cwarnings, build)
	if err != nil {
		return "", err
	}

	broker.Logger.Info(fmt.Sprintf("RS %v => polling build", instanceId))
	droplet, pbwarnings, err := broker.pollBuild(build.GUID, appName)
	broker.showWarnings(pbwarnings, droplet)
	if err != nil {
		return "", err
	}

	broker.Logger.Info(fmt.Sprintf("RS %v => set application droplet", instanceId))
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
		return "", errors.New(fmt.Sprintf("RS %v => no domains found for this instance", instanceId))
	}

	route, _, err := cfClient.CreateRoute(resources.Route{
		SpaceGUID:  spaceGUID,
		DomainGUID: domains[0].GUID,
		Host:       appName,
	})
	if err != nil {
		return "", err
	}

	_, err = cfClient.MapRoute(route.GUID, app.GUID)

	if err != nil {
		return "", err
	}

	time.Sleep(time.Second)

	broker.Logger.Info(fmt.Sprintf("RS %v => Starting Application", instanceId))
	app, _, err = cfClient.UpdateApplicationStart(app.GUID)
	if err != nil {
		broker.Logger.Info(fmt.Sprintf("RS %v => Application Start Failed, Trying restart", instanceId))
		app, _, err = cfClient.UpdateApplicationRestart(app.GUID)
		if err != nil {
			broker.Logger.Info(fmt.Sprintf("RS %v => Application Start failed", instanceId))
			return "", err
		}
	}

	broker.Logger.Info(fmt.Sprintf("RS %v => handling node count", instanceId))
	// handle the node count
	if count > 1 {
		rc.Clustered()
		broker.Logger.Info(fmt.Sprintf("RS %v => scaling to %d", instanceId, count))
		err = broker.scaleRegistryServer(cfClient, &app, count)
		if err != nil {
			return "", err
		}

		community, err := broker.GetCommunity()
		if err != nil {
			return "", err
		}

		stats, err := getProcessStatsByAppAndType(cfClient, community, broker.Logger, app.GUID, "web")
		if err != nil {
			return "", nil
		}

		for _, stat := range stats {

			rc.AddPeer(stat.Index, fmt.Sprintf("http://%s:%d/eureka", stat.Host, stat.InstancePorts[0].External), serviceId)
		}
	} else {
		rc.Standalone()
	}

	broker.Logger.Info(fmt.Sprintf("RS %v => Updating Environment", instanceId))
	err = broker.UpdateRegistryEnvironment(cfClient, &app, &info, serviceId, instanceId, rc, params)

	if err != nil {
		return "", err
	}

	broker.Logger.Info(fmt.Sprintf("RS %v => Starting Application", instanceId))
	app, _, err = cfClient.UpdateApplicationStart(app.GUID)
	if err != nil {
		broker.Logger.Info(fmt.Sprintf("RS %v => Application Start Failed, Trying restart", instanceId))
		app, _, err = cfClient.UpdateApplicationRestart(app.GUID)
		if err != nil {
			broker.Logger.Info(fmt.Sprintf("RS %v => Application Start failed", instanceId))
			return "", err
		}
	}

	community, err := broker.GetCommunity()
	if err != nil {
		return "", err
	}

	if count > 1 {
		stats, err := getProcessStatsByAppAndType(cfClient, community, broker.Logger, app.GUID, "web")
		if err != nil {
			return "", err
		}

		for _, stat := range stats {
			rc.AddPeer(stat.Index, fmt.Sprintf("http://%s:%d/eureka", stat.Host, stat.InstancePorts[0].External), serviceId)
		}
	}

	peers, err := json.Marshal(rc.Peers)
	if err != nil {
		return "", err
	}
	x := 0
	for _, peer := range rc.Peers {
		req, err := http.NewRequest(http.MethodPost, "https://"+route.URL+"/config/peers", bytes.NewBuffer(peers))
		if err != nil {
			fmt.Printf("RS client: could not create request: %s\n", err)

		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Cf-App-Instance", app.GUID+":"+strconv.Itoa(peer.Index))

		refreshreq, err := http.NewRequest(http.MethodPost, "https://"+route.URL+"/actuator/refresh", nil)
		if err != nil {
			fmt.Printf("RS client: could not create request: %s\n", err)

		}
		refreshreq.Header.Set("Content-Type", "application/json")
		refreshreq.Header.Set("X-Cf-App-Instance", app.GUID+":"+strconv.Itoa(peer.Index))

		client := http.Client{
			Timeout: 30 * time.Second,
		}

		res, err := client.Do(req)
		if err != nil {
			fmt.Printf("RS client: error making http request: %s\n", err)
		}
		broker.Logger.Info(res.Request.RequestURI)
		broker.Logger.Info(string(peers))
		broker.Logger.Info(res.Status)

		refreshres, err := client.Do(refreshreq)
		if err != nil {
			fmt.Printf("RS client: error making http request: %s\n", err)
		}
		broker.Logger.Info(refreshres.Request.RequestURI)
		broker.Logger.Info(string(peers))
		broker.Logger.Info(refreshres.Status)
		x++
	}

	broker.Logger.Info(route.URL)

	successfulStart, err := broker.MonitorApplicationStartup(cfClient, community, broker.Logger, app.GUID)
	if err != nil || !successfulStart {
		broker.Logger.Info(fmt.Sprintf("RS %v => Crashed application restarting...", instanceId))
		app, _, err = cfClient.UpdateApplicationStart(app.GUID)
		if err != nil {
			broker.Logger.Info(fmt.Sprintf("RS %v => Application Start Failed, Trying restart", instanceId))
			app, _, err = cfClient.UpdateApplicationRestart(app.GUID)
			if err != nil {
				broker.Logger.Info(fmt.Sprintf("RS %v => Application Start failed", instanceId))
				return "", err
			}
		}
	}

	return route.URL, nil
}
