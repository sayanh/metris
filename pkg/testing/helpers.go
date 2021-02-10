package testing

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"time"

	corev1 "k8s.io/api/core/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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

type NewShootOpts func(shoot *gardencorev1beta1.Shoot)

func GetShoot(name string, opts ...NewShootOpts) *gardencorev1beta1.Shoot {
	shoot := &gardencorev1beta1.Shoot{
		TypeMeta: metaV1.TypeMeta{
			Kind:       "Shoot",
			APIVersion: "core.gardener.cloud/v1beta1",
		},
		ObjectMeta: metaV1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: gardencorev1beta1.ShootSpec{},
	}
	for _, opt := range opts {
		opt(shoot)
	}
	return shoot
}

func WithAzureProviderAndStandard_D8_v3VMs(shoot *gardencorev1beta1.Shoot) {
	shoot.Spec.Provider = gardencorev1beta1.Provider{
		Type:                 "azure",
		ControlPlaneConfig:   nil,
		InfrastructureConfig: nil,
		Workers: []gardencorev1beta1.Worker{
			{
				Name: "cpu-worker-0",
				Machine: gardencorev1beta1.Machine{
					Type: "Standard_D8_v3",
					Image: &gardencorev1beta1.ShootMachineImage{
						Name: "gardenlinux",
					},
				},
			},
		},
	}
}

func WithAzureProviderAndFooVMType(shoot *gardencorev1beta1.Shoot) {
	shoot.Spec.Provider = gardencorev1beta1.Provider{
		Type:                 "azure",
		ControlPlaneConfig:   nil,
		InfrastructureConfig: nil,
		Workers: []gardencorev1beta1.Worker{
			{
				Name: "cpu-worker-0",
				Machine: gardencorev1beta1.Machine{
					Type: "Standard_Foo",
					Image: &gardencorev1beta1.ShootMachineImage{
						Name: "gardenlinux",
					},
				},
			},
		},
	}
}

func Get2Nodes() *corev1.NodeList {
	node1 := GetNode("node1", "Standard_D8_v3")
	node2 := GetNode("node2", "Standard_D8_v3")
	return &corev1.NodeList{
		Items: []corev1.Node{node1, node2},
	}
}

func Get3NodesWithStandardD8v3VMType() *corev1.NodeList {
	node1 := GetNode("node1", "Standard_D8_v3")
	node2 := GetNode("node2", "Standard_D8_v3")
	node3 := GetNode("node3", "Standard_D8_v3")
	return &corev1.NodeList{
		TypeMeta: metaV1.TypeMeta{
			Kind:       "NodeList",
			APIVersion: "v1",
		},
		Items: []corev1.Node{node1, node2, node3},
	}
}

func Get3NodesWithFooVMType() *corev1.NodeList {
	node1 := GetNode("node1", "foo")
	node2 := GetNode("node2", "foo")
	node3 := GetNode("node3", "foo")
	return &corev1.NodeList{
		Items: []corev1.Node{node1, node2, node3},
	}
}

func GetNode(name, vmType string) corev1.Node {
	return corev1.Node{
		TypeMeta: metaV1.TypeMeta{
			Kind:       "Node",
			APIVersion: "v1",
		},
		ObjectMeta: metaV1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"node.kubernetes.io/instance-type": vmType,
				"node.kubernetes.io/role":          "node",
			},
		},
	}
}
