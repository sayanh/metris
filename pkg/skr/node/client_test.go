package node

import (
	"context"
	"testing"

	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1 "k8s.io/api/core/v1"

	"github.com/kyma-incubator/metris/pkg/gardener/commons"
	metristesting "github.com/kyma-incubator/metris/pkg/testing"
	"github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func TestList(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	ctx := context.Background()

	nodeList := metristesting.Get3NodesWithStandardD8v3VMType()
	client, err := NewFakeClient(nodeList)
	g.Expect(err).Should(gomega.BeNil())

	gotNodeList, err := client.List(ctx)
	g.Expect(err).Should(gomega.BeNil())
	g.Expect(len(gotNodeList.Items)).To(gomega.Equal(len(nodeList.Items)))
	g.Expect(*gotNodeList).To(gomega.Equal(*nodeList))

	// Delete all the nodes
	for _, node := range nodeList.Items {
		err := client.Resource.Delete(ctx, node.Name, metaV1.DeleteOptions{})
		g.Expect(err).Should(gomega.BeNil())
	}

	gotNodeList, err = client.List(ctx)
	g.Expect(err).Should(gomega.BeNil())
	g.Expect(len(gotNodeList.Items)).To(gomega.Equal(0))
}

func NewFakeClient(nodeList *corev1.NodeList) (*Client, error) {
	scheme, err := commons.SetupSchemeOrDie()
	if err != nil {
		return nil, err
	}
	unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(nodeList)
	if err != nil {
		return nil, err
	}
	nodeListUnstructured := &unstructured.UnstructuredList{Object: unstructuredMap}
	nodeListUnstructured.SetUnstructuredContent(unstructuredMap)
	nodeListUnstructured.SetGroupVersionKind(GroupVersionKind())
	nodeListUnstructured.SetKind("NodeList")
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme, nodeListUnstructured)
	nsResourceClient := dynamicClient.Resource(GroupVersionResource())

	return &Client{Resource: nsResourceClient}, nil
}
