package process

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"k8s.io/apimachinery/pkg/runtime"

	corev1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/onsi/gomega"

	dynamicFake "k8s.io/client-go/dynamic/fake"

	metristesting "github.com/kyma-incubator/metris/pkg/testing"
	kebruntime "github.com/kyma-project/control-plane/components/kyma-environment-broker/common/runtime"
)

func TestFilterRuntimes(t *testing.T) {
	_ = []struct {
		name             string
		inputRuntimes    kebruntime.RuntimesPage
		expectedRuntimes kebruntime.RuntimesPage
	}{
		{
			name: "filter out 1 RuntimesPage from 2",
			inputRuntimes: kebruntime.RuntimesPage{
				Data: []kebruntime.RuntimeDTO{
					metristesting.NewRuntimesDTO("foo1", metristesting.WithSucceededState),
					metristesting.NewRuntimesDTO("foo2", metristesting.WithFailedState),
				},
				Count:      2,
				TotalCount: 2,
			},
			expectedRuntimes: kebruntime.RuntimesPage{
				Data: []kebruntime.RuntimeDTO{
					metristesting.NewRuntimesDTO("foo1", metristesting.WithSucceededState),
				},
				Count:      1,
				TotalCount: 1,
			},
		},
		{
			name: "filter out 0 RuntimesPage from 2",
			inputRuntimes: kebruntime.RuntimesPage{
				Data: []kebruntime.RuntimeDTO{
					metristesting.NewRuntimesDTO("foo1", metristesting.WithFailedState),
					metristesting.NewRuntimesDTO("foo2", metristesting.WithFailedState),
				},
				Count:      2,
				TotalCount: 2,
			},
			expectedRuntimes: kebruntime.RuntimesPage{
				Data:       nil,
				Count:      0,
				TotalCount: 0,
			},
		},
		{
			name: "filter out 2 RuntimesPage from 2",
			inputRuntimes: kebruntime.RuntimesPage{
				Data: []kebruntime.RuntimeDTO{
					metristesting.NewRuntimesDTO("foo1", metristesting.WithSucceededState),
					metristesting.NewRuntimesDTO("foo2", metristesting.WithSucceededState),
				},
				Count:      2,
				TotalCount: 2,
			},
			expectedRuntimes: kebruntime.RuntimesPage{
				Data: []kebruntime.RuntimeDTO{
					metristesting.NewRuntimesDTO("foo1", metristesting.WithSucceededState),
					metristesting.NewRuntimesDTO("foo2", metristesting.WithSucceededState),
				},
				Count:      2,
				TotalCount: 2,
			},
		},
	}

	//for _, tc := range testCases {
	//	t.Run(tc.name, func(t *testing.T) {
	//		gotRuntimesPage := (tc.inputRuntimes)
	//		assert.Equal(t, tc.expectedRuntimes, *gotRuntimesPage, "filteredRuntimes should return only the ones with succeeded")
	//	})
	//}
}

func TestGetNodes(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	fakeDynClient := new(dynamicFake.FakeDynamicClient)
	ctx := context.Background()

	node1 := corev1.Node{
		ObjectMeta: metaV1.ObjectMeta{
			Name: "node1",
		},
		Spec: corev1.NodeSpec{},
	}
	//node1Map, err := runtime.DefaultUnstructuredConverter.ToUnstructured(node1)
	//g.Expect(err).Should(gomega.BeNil())
	unstructuredNode, err := toUnstructured(&node1)
	g.Expect(err).Should(gomega.BeNil())
	_, err = fakeDynClient.Resource(nodeGVR).Create(ctx, unstructuredNode, metaV1.CreateOptions{})
	g.Expect(err).Should(gomega.BeNil())

	nodes, err := getNodes(ctx, fakeDynClient)
	g.Expect(err).Should(gomega.BeNil())
	g.Expect(len(nodes.Items)).To(gomega.Equal(1))

}

func toUnstructured(obj interface{}) (*unstructured.Unstructured, error) {
	object, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)

	if err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{Object: object}, nil
}

//func TestGetShoots(t *testing.T)

//func TestGetSecretForShoot(t *testing.T)
