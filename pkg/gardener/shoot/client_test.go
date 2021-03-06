package shoot

import (
	"context"
	"testing"

	metristesting "github.com/kyma-incubator/metris/pkg/testing"

	"github.com/onsi/gomega"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"

	gardenerv1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/kyma-incubator/metris/pkg/gardener/commons"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func TestGet(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	ctx := context.Background()

	shoot := metristesting.GetShoot("foo-shoot", metristesting.WithVMSpecs)
	nsResourceClient, err := NewFakeClient(shoot)
	g.Expect(err).Should(gomega.BeNil())
	client := Client{ResourceClient: nsResourceClient}

	gotShoot, err := client.Get(ctx, "foo-shoot")
	g.Expect(err).Should(gomega.BeNil())
	g.Expect(*gotShoot).To(gomega.Equal(*shoot))

	gotShoot, err = client.Get(ctx, "doesnotexist-shoot")
	g.Expect(err).ShouldNot(gomega.BeNil())
	g.Expect(k8sErrors.IsNotFound(err)).To(gomega.BeTrue())
}

func NewFakeClient(shoot *gardenerv1beta1.Shoot) (dynamic.ResourceInterface, error) {
	scheme, err := commons.SetupSchemeOrDie()
	if err != nil {
		return nil, err
	}
	unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(shoot)
	if err != nil {
		return nil, err
	}
	secretUnstructured := &unstructured.Unstructured{Object: unstructuredMap}
	secretUnstructured.SetGroupVersionKind(GroupVersionKind())

	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme, secretUnstructured)
	nsResourceClient := dynamicClient.Resource(GroupVersionResource()).Namespace("default")

	return nsResourceClient, nil
}
