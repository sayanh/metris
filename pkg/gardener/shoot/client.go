package shoot

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"

	gardenerv1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/kyma-incubator/metris/options"
	gardenercommons "github.com/kyma-incubator/metris/pkg/gardener/commons"
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
		return nil, err
	}
	restConfig := dynamic.ConfigFor(client)
	dynClient := dynamic.NewForConfigOrDie(restConfig)
	resourceClient := dynClient.Resource(GroupVersionResource()).Namespace(opts.GardenerNamespace)
	return &Client{resourceClient: resourceClient}, nil
}

func GroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Version:  gardenerv1beta1.SchemeGroupVersion.Version,
		Group:    gardenerv1beta1.SchemeGroupVersion.Group,
		Resource: "shoots",
	}
}

func (c Client) Get(ctx context.Context, shootName string) (*gardenerv1beta1.Shoot, error) {
	unstructuredShoot, err := c.resourceClient.Get(ctx, shootName, metaV1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return convertRuntimeObjToShoot(unstructuredShoot)
}

func convertRuntimeObjToShoot(shootUnstructured *unstructured.Unstructured) (*gardenerv1beta1.Shoot, error) {
	shoot := new(gardenerv1beta1.Shoot)
	err := k8sruntime.DefaultUnstructuredConverter.FromUnstructured(shootUnstructured.Object, shoot)
	if err != nil {
		return nil, err
	}
	return shoot, nil
}
