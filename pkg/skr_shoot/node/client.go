package node

import (
	"context"
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
)

type Client struct {
	lister dynamic.Interface
}

func NewClient(kubeconfig string) (*Client, error) {
	restClientConfig, err := clientcmd.RESTConfigFromKubeConfig([]byte(kubeconfig))
	if err != nil {
		return nil, err
	}
	dynamicClient, err := dynamic.NewForConfig(restClientConfig)
	return &Client{lister: dynamicClient}, nil
}

func GroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Version:  corev1.SchemeGroupVersion.Version,
		Group:    corev1.SchemeGroupVersion.Group,
		Resource: "nodes",
	}
}

func (c Client) List(ctx context.Context) (*corev1.NodeList, error) {

	nodeClient := c.lister.Resource(GroupVersionResource())
	nodesUnstructured, err := nodeClient.List(ctx, metaV1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return convertRuntimeListToNodeList(nodesUnstructured)
}

func convertRuntimeListToNodeList(unstructuredNodesList *unstructured.UnstructuredList) (*corev1.NodeList, error) {
	nodeList := new(corev1.NodeList)
	nodeListBytes, err := unstructuredNodesList.MarshalJSON()
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(nodeListBytes, nodeList)
	if err != nil {
		return nil, err
	}
	return nodeList, nil
}
