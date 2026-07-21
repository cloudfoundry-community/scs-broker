package broker

import (
	"context"
	"errors"

	brokerapi "code.cloudfoundry.org/brokerapi/v13/domain"
)

func (broker *SCSBroker) LastOperation(ctx context.Context, instanceID string, details brokerapi.PollDetails) (brokerapi.LastOperation, error) {
	return brokerapi.LastOperation{}, errors.New("not implemented")
}

func (broker *SCSBroker) GetBinding(ctx context.Context, instanceID, bindingID string, details brokerapi.FetchBindingDetails) (brokerapi.GetBindingSpec, error) {
	return brokerapi.GetBindingSpec{}, errors.New("not implemented")
}

func (broker *SCSBroker) GetInstance(ctx context.Context, instanceID string, details brokerapi.FetchInstanceDetails) (brokerapi.GetInstanceDetailsSpec, error) {
	return brokerapi.GetInstanceDetailsSpec{}, errors.New("not implemented")
}

func (broker *SCSBroker) LastBindingOperation(ctx context.Context, instanceID, bindingID string, details brokerapi.PollDetails) (brokerapi.LastOperation, error) {
	return brokerapi.LastOperation{}, errors.New("not implemented")
}
