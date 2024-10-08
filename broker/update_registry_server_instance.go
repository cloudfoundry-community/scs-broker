package broker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"code.cloudfoundry.org/cli/api/cloudcontroller/ccv3"
	"code.cloudfoundry.org/lager"
	"github.com/cloudfoundry-community/scs-broker/broker/utilities"
	scsccparser "github.com/cloudfoundry-community/spring-cloud-services-cli-config-parser"
	brokerapi "github.com/pivotal-cf/brokerapi/domain"
)

func (broker *SCSBroker) updateRegistryServerInstance(cxt context.Context, instanceID string, details brokerapi.UpdateDetails, asyncAllowed bool) (brokerapi.UpdateServiceSpec, error) {
	spec := brokerapi.UpdateServiceSpec{}

	appName := utilities.MakeAppName(details.ServiceID, instanceID)
	spaceGUID := broker.Config.InstanceSpaceGUID

	broker.Logger.Info("broker.UpdateRegistryServerInstance: update-service-instance", lager.Data{"plan-id": details.PlanID, "service-id": details.ServiceID})
	envsetup := scsccparser.EnvironmentSetup{}
	cfClient, err := broker.GetClient()
	if err != nil {
		broker.Logger.Error("broker.UpdateRegistryServerInstance: broker.GetClient()", err)
		return spec, errors.New("Couldn't start session: " + err.Error())
	}

	community, err := broker.GetCommunity()
	if err != nil {
		broker.Logger.Error("broker.UpdateRegistryServerInstance: broker.GetCommunity()", err)
		return spec, err
	}

	rc := utilities.NewRegistryConfig()
	rp, err := utilities.ExtractRegistryParams(string(details.RawParameters))
	if err != nil {
		broker.Logger.Error("broker.UpdateRegistryServerInstance: utilities.ExtractRegistryParams()", err)
		return spec, err
	}

	count, err := rp.Count()
	if err != nil {
		broker.Logger.Error("broker.UpdateRegistryServerInstance: rp.Count()", err)
		return spec, err
	}

	info, _, _, err := cfClient.GetInfo()
	if err != nil {
		broker.Logger.Error("broker.UpdateRegistryServerInstance: cfClient.GetInfo()", err)
		return spec, err
	}

	app, _, err := cfClient.GetApplicationByNameAndSpace(appName, spaceGUID)
	if err != nil {
		broker.Logger.Error("broker.UpdateRegistryServerInstance: cfClient.GetApplicationByNameAndSpace()", err)
		return spec, errors.New("Couldn't find app session: " + err.Error())
	}

	mapparams, err := envsetup.ParseEnvironmentFromRaw(details.RawParameters)
	if err != nil {
		broker.Logger.Error("broker.UpdateRegistryServerInstance: cfClient.ParseEnvironmentFromRaw()", err)
		return spec, err
	}

	broker.Logger.Info("Updating Environment")
	err = broker.UpdateAppEnvironment(cfClient, &app, &info, details.ServiceID, instanceID, string(details.RawParameters), mapparams)
	if err != nil {
		broker.Logger.Error("broker.UpdateRegistryServerInstance: cfClient.UpdateAppEnvironment()", err)
		return spec, err
	}

	broker.Logger.Info("Updating application")

	_, _, err = cfClient.UpdateApplication(utilities.SafeApp(app))
	if err != nil {
		broker.Logger.Error("broker.UpdateRegistryServerInstance: cfClient.UpdateApplication()", err)
		return spec, err
	}

	broker.Logger.Info("broker.UpdateRegistryServerInstance: handling node count")
	// handle the node count
	if count > 1 {
		rc.Clustered()
	} else {
		rc.Standalone()
	}

	// since this is an update, we need to scale, but only if the desired proc
	// count has changed
	procs, err := getApplicationProcessesByType(cfClient, broker.Logger, app.GUID, "web")
	if err != nil {
		broker.Logger.Error("broker.UpdateRegistryServerInstance: getApplicationProcessesByType()", err)
		return spec, err
	}

	procCount := 0
	for _, proc := range procs {
		if proc.Instances.IsSet {
			procCount += proc.Instances.Value
		}
	}

	broker.Logger.Info(fmt.Sprintf("I received %d procs from the API", procCount))

	if count != procCount {
		broker.Logger.Info(fmt.Sprintf("Scaling to %d procs", count))
		err = broker.scaleRegistryServer(cfClient, &app, count)
		if err != nil {
			broker.Logger.Error("broker.UpdateRegistryServerInstance: broker.scaleRegistryServer()", err)
			return spec, err
		}
	}

	if count > 1 {
		stats, err := getProcessStatsByAppAndType(cfClient, community, broker.Logger, app.GUID, "web")
		if err != nil {
			broker.Logger.Error("broker.UpdateRegistryServerInstance: getProcessStatsByAppAndType()", err)
			return spec, err
		}

		for _, stat := range stats {
			rc.AddPeer(stat.Index, fmt.Sprintf("http://%s:%d/eureka", stat.Host, stat.InstancePorts[0].External), details.ServiceID)
		}
	}

	broker.Logger.Info("Updating Environment")
	err = broker.UpdateRegistryEnvironment(cfClient, &app, &info, details.ServiceID, instanceID, rc, mapparams)
	if err != nil {
		broker.Logger.Error("broker.UpdateRegistryServerInstance: broker.UpdateRegistryEnvironment()", err)
		return spec, err
	}

	app, _, err = cfClient.UpdateApplicationRestart(app.GUID)
	if err != nil {
		broker.Logger.Error("broker.UpdateRegistryServerInstance: cfClient.UpdateApplicationRestart()", err)
		return spec, err
	}

	route, _, err := cfClient.GetApplicationRoutes(app.GUID)
	if err != nil {
		broker.Logger.Error("broker.UpdateRegistryServerInstance: cfClient.GetApplicationRoutes()", err)
		// TODO: Why was there no return here???
	}

	peers, err := json.Marshal(rc.Peers)
	if err != nil {
		broker.Logger.Error("broker.UpdateRegistryServerInstance: json.Marshal()", err)
		return spec, err
	}

	x := 0
	baseURL := fmt.Sprintf("https://%s", route[0].URL)
	for _, peer := range rc.Peers {
		req, err := http.NewRequest(http.MethodPost, baseURL+"/config/peers", bytes.NewBuffer(peers))
		if err != nil {
			broker.Logger.Error("broker.UpdateRegistryServerInstance: http.NewRequest()", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Cf-App-Instance", app.GUID+":"+strconv.Itoa(peer.Index))

		refreshreq, err := http.NewRequest(http.MethodPost, baseURL+"/actuator/refresh", nil)
		if err != nil {
			broker.Logger.Error("broker.UpdateRegistryServerInstance: http.NewRequest()", err)
		}
		refreshreq.Header.Set("Content-Type", "application/json")
		refreshreq.Header.Set("X-Cf-App-Instance", app.GUID+":"+strconv.Itoa(peer.Index))

		client := http.Client{
			Timeout: 30 * time.Second,
		}

		res, err := client.Do(req)
		if err != nil {
			broker.Logger.Error("broker.UpdateRegistryServerInstance: http.Client.Do()", err)
		}
		broker.Logger.Info(res.Request.RequestURI)
		broker.Logger.Info(string(peers))
		broker.Logger.Info(res.Status)

		refreshres, err := client.Do(refreshreq)
		if err != nil {
			broker.Logger.Error("broker.UpdateRegistryServerInstance: http.Client.Do()", err)
		}
		broker.Logger.Info(refreshres.Request.RequestURI)
		broker.Logger.Info(string(peers))
		broker.Logger.Info(refreshres.Status)
		x++
	}

	return spec, nil
}

func getApplicationProcessesByType(client *ccv3.Client, logger lager.Logger, appGUID string, procType string) ([]ccv3.Process, error) {
	filtered := make([]ccv3.Process, 0)

	candidates, _, err := client.GetApplicationProcesses(appGUID)
	if err != nil {
		return filtered, err
	}

	logger.Info(fmt.Sprintf("broker.UpdateRegistryServerInstance: client.getApplicationProcessesByType() got %d total procs", len(candidates)))

	for _, prospect := range candidates {
		if prospect.Type == procType {
			filtered = append(filtered, prospect)
		}
	}

	return filtered, nil
}
