package broker

import (
	"errors"
	"time"

	"code.cloudfoundry.org/cli/api/cloudcontroller/ccv3"
	"code.cloudfoundry.org/cli/api/cloudcontroller/ccv3/constant"
	"code.cloudfoundry.org/cli/resources"
	"code.cloudfoundry.org/cli/util/configv3"
	"code.cloudfoundry.org/lager"
)

func (broker *SCSBroker) pollBuild(buildGUID string, appName string) (resources.Droplet, ccv3.Warnings, error) {
	var allWarnings ccv3.Warnings

	timeout := time.After(configv3.DefaultStagingTimeout)
	interval := time.NewTimer(0)

	cfClient, err := broker.GetClient()
	if err != nil {
		return resources.Droplet{}, nil, errors.New("couldn't start session: " + err.Error())
	}

	for {
		select {
		case <-interval.C:
			build, warnings, err := cfClient.GetBuild(buildGUID)
			allWarnings = append(allWarnings, warnings...)
			if err != nil {
				return resources.Droplet{}, allWarnings, err
			}

			broker.Logger.Info("polling build final state:", lager.Data{
				"package_guid": build.GUID,
				"state":        build.State,
			})

			switch build.State {
			case constant.BuildFailed:
				return resources.Droplet{}, allWarnings, errors.New(build.Error)

			case constant.BuildStaged:
				droplet, warnings, err := cfClient.GetDroplet(build.DropletGUID)
				allWarnings = append(allWarnings, warnings...)
				if err != nil {
					return resources.Droplet{}, allWarnings, err
				}

				return resources.Droplet{
					GUID:      droplet.GUID,
					State:     droplet.State,
					CreatedAt: droplet.CreatedAt,
				}, allWarnings, nil
			}

			interval.Reset(configv3.DefaultPollingInterval)

		case <-timeout:
			return resources.Droplet{}, allWarnings, errors.New("staging timed out")
		}
	}
}
