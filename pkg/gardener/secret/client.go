package secret

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"

	"github.com/kyma-incubator/metris/options"

	gardenercommons "github.com/kyma-incubator/metris/pkg/gardener/commons"
	corev1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

type Client struct {
	resourceClient dynamic.ResourceInterface
}

func NewClient(opts *options.Options) (*Client, error) {
	k8sConfig := gardenercommons.GetGardenerKubeconfig(opts)
	client, err := k8sConfig.ClientConfig()
	if err != nil {
		return nil, nil
	}
	restConfig := dynamic.ConfigFor(client)
	dynClient := dynamic.NewForConfigOrDie(restConfig)
	resourceClient := dynClient.Resource(GroupVersionResource()).Namespace(opts.GardenerNamespace)
	return &Client{resourceClient: resourceClient}, nil
}

func GroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Version:  corev1.SchemeGroupVersion.Version,
		Group:    corev1.SchemeGroupVersion.Group,
		Resource: "secrets",
	}
}

func (c Client) Get(ctx context.Context, shootName string) (*corev1.Secret, error) {
	shootKubeconfigName := fmt.Sprintf("%s.kubeconfig", shootName)
	unstructuredSecret, err := c.resourceClient.Get(ctx, shootKubeconfigName, metaV1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return convertRuntimeObjToSecret(unstructuredSecret)
}

func convertRuntimeObjToSecret(unstructuredSecret *unstructured.Unstructured) (*corev1.Secret, error) {
	secret := new(corev1.Secret)
	err := k8sruntime.DefaultUnstructuredConverter.FromUnstructured(unstructuredSecret.Object, secret)
	if err != nil {
		return nil, err
	}
	return secret, nil
}
