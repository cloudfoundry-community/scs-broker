package broker

import (
	"context"
	"fmt"

	scsccparser "github.com/cloudfoundry-community/spring-cloud-services-cli-config-parser"
	brokerapi "github.com/pivotal-cf/brokerapi/domain"
)

func (broker *SCSBroker) CreateServiceInstances(ctx context.Context, instanceID string, details brokerapi.ProvisionDetails, asyncAllowed bool) error {
	broker.Logger.Info(fmt.Sprintf("Starting goroutine for creating service application instance id: %s", instanceID))

	go broker.startInstances(instanceID, details)

	return nil
}

func (broker *SCSBroker) startInstances(instanceID string, details brokerapi.ProvisionDetails) (string, error) {
	var provisioner func(string, string, string, map[string]string) (string, error)

	envsetup := scsccparser.EnvironmentSetup{}
	raw := details.RawParameters
	if len(raw) == 0 {
		raw = []byte("{}")
	}

	mapparams, err := envsetup.ParseEnvironmentFromRaw(raw)
	if err != nil {
		return "", err
	}

	switch details.ServiceID {
	case "service-registry":
		provisioner = broker.createRegistryServerInstance
	case "config-server":
		provisioner = broker.createConfigServerInstance
	}

	url, err := provisioner(details.ServiceID, instanceID, string(details.RawParameters), mapparams)
	if err != nil {
		return "", err
	}

	return url, nil
}
