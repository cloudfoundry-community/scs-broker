package broker

import (
	"context"
	"fmt"

	brokerapi "code.cloudfoundry.org/brokerapi/v13/domain"
	"github.com/cloudfoundry-community/scs-broker/broker/utilities"
)

func (broker *SCSBroker) Unbind(ctx context.Context, instanceID, bindingID string, details brokerapi.UnbindDetails, asyncAllowed bool) (brokerapi.UnbindSpec, error) {
	unbind := brokerapi.UnbindSpec{}

	broker.Logger.Info("broker.UnBind: GetUAAClient")
	api, err := broker.GetUaaClient()
	if err != nil {
		broker.Logger.Error("broker.UnBind: broker.GetUaaClient()", err)
		return unbind, err
	}

	broker.Logger.Info("broker.UnBind: makeClientIdForBinding")
	clientId := utilities.MakeClientIdForBinding(details.ServiceID, bindingID)

	broker.Logger.Info(fmt.Sprintf("broker.UnBind: DeleteClient bindingID:%s clientid %s", bindingID, clientId))
	_, err = api.DeleteClient(clientId)
	if err != nil {
		broker.Logger.Error("broker.UnBind: api.DeleteClient()", err)
		return unbind, nil
	}
	broker.Logger.Info("broker.UnBind: Return")
	return unbind, nil
}
