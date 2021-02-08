package testing

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/onsi/gomega"

	"github.com/gorilla/mux"

	kebruntime "github.com/kyma-project/control-plane/components/kyma-environment-broker/common/runtime"
)

const (
	providersFile = "static_providers.json"
	timeout       = 10 * time.Second
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

func LoadFixtureFromFile(filePath string) ([]byte, error) {
	return ioutil.ReadFile(filePath)
}

func StartTestServer(path string, testHandler http.HandlerFunc, g gomega.Gomega) *httptest.Server {
	testRouter := mux.NewRouter()
	testRouter.HandleFunc("/health", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
	}).Methods(http.MethodGet)
	testRouter.HandleFunc(path, testHandler)

	// Start a local test HTTP server
	srv := httptest.NewServer(testRouter)

	// Wait until test server is ready
	g.Eventually(func() int {
		// Ignoring error is ok as it goes for retry for non-200 cases
		healthResp, _ := http.Get(fmt.Sprintf("%s/health", srv.URL))
		return healthResp.StatusCode
	}, timeout).Should(gomega.Equal(http.StatusOK))

	return srv
}
