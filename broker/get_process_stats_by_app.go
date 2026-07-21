package broker

import (
	"context"
	"errors"

	"code.cloudfoundry.org/cli/v8/api/cloudcontroller/ccv3"
	"code.cloudfoundry.org/lager/v3"
	cfclient "github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
)

func getProcessStatsByAppAndType(ctx context.Context, cfClient *ccv3.Client, community *cfclient.Client, logger lager.Logger, appGUID string, procType string) ([]resource.ProcessStat, error) {
	stats := make([]resource.ProcessStat, 0)

	procs, err := getApplicationProcessesByType(cfClient, logger, appGUID, procType)
	if err != nil {
		return stats, err
	}

	for _, proc := range procs {
		candidates, err := community.Processes.GetStats(ctx, proc.GUID)
		if err != nil {
			continue
		}

		stats = append(stats, candidates.Stats...)
	}

	if len(stats) == 0 {
		return stats, errors.New("no stats found")
	}

	return stats, nil

}
