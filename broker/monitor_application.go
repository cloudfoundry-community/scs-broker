package broker

import (
	"time"

	"code.cloudfoundry.org/cli/api/cloudcontroller/ccv3"
	"github.com/cloudfoundry-community/go-cfclient/v2"
)

func (broker *SCSBroker) MonitorApplicationStartup(cfClient *ccv3.Client, community *cfclient.Client, appGUID string) (bool, error) {

	waittime := 30
	timepassed := 0

	for timepassed < waittime {
		time.Sleep(time.Second)
		timepassed += 1
		successStart, err := broker.checkApplicationStatus(cfClient, community, appGUID)
		if err != nil {
			return false, err
		}
		if !successStart {
			return successStart, err
		}
	}

	return true, nil

}

func (broker *SCSBroker) checkApplicationStatus(cfClient *ccv3.Client, community *cfclient.Client, appGUID string) (bool, error) {
	stats, err := getProcessStatsByAppAndType(cfClient, community, broker.Logger, appGUID, "web")
	if err != nil {
		broker.Logger.Error("broker.MonitorApplication: getProcessStatsByAppAndType()", err)
		return false, err
	}

	for _, stat := range stats {
		if stat.State == "CRASHED" {
			broker.Logger.Error("broker.MonitorApplication: App CRASHED State.", err)
			return false, err
		}
	}

	return true, nil
}
