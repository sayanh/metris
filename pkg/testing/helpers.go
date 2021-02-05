package testing

import (
	"io/ioutil"

	kebruntime "github.com/kyma-project/control-plane/components/kyma-environment-broker/common/runtime"
)

const (
	providersFile = "static_providers.json"
)

type NewRuntimeOpts func(*kebruntime.RuntimeDTO)

func NewRuntimesDTO(shootName string, opts ...NewRuntimeOpts) kebruntime.RuntimeDTO {
	runtime := kebruntime.RuntimeDTO{
		ShootName: shootName,
		Status: kebruntime.RuntimeStatus{
			Provisioning: &kebruntime.Operation{
				State: "succeeded",
			},
		},
	}

	for _, opt := range opts {
		opt(&runtime)
	}

	return runtime
}

func WithSucceededState(runtime *kebruntime.RuntimeDTO) {
	runtime.Status.Provisioning.State = "succeeded"
}
func WithFailedState(runtime *kebruntime.RuntimeDTO) {
	runtime.Status.Provisioning.State = "failed"
}

func LoadProvidersFromFile(filePath string) ([]byte, error) {
	return ioutil.ReadFile(filePath)
}
