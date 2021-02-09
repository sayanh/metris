package process

import (
	"context"
	"fmt"
	"net/http"

	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/util/json"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	corev1 "k8s.io/api/core/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"

	gardenerv1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

var (
	nodeGVR = schema.GroupVersionResource{
		Version:  corev1.SchemeGroupVersion.Version,
		Group:    corev1.SchemeGroupVersion.Group,
		Resource: "nodes",
	}
)

//func filterRuntimes(runtimesPage kebruntime.RuntimesPage) *kebruntime.RuntimesPage {
//
//	// TODO check for deprovisioning as well
//	filteredRuntimes := new(kebruntime.RuntimesPage)
//	for _, runtime := range runtimesPage.Data {
//		if runtime.Status.Provisioning.State == "succeeded" {
//			filteredRuntimes.Data = append(filteredRuntimes.Data, runtime)
//		}
//	}
//	filteredRuntimes.Count = len(filteredRuntimes.Data)
//	filteredRuntimes.TotalCount = len(filteredRuntimes.Data)
//
//	return filteredRuntimes
//
//}

func isSuccess(status int) bool {
	if status >= http.StatusOK && status < http.StatusMultipleChoices {
		return true
	}
	return false
}

func convertRuntimeObjToShoot(shootUnstructured *unstructured.Unstructured) (*gardencorev1beta1.Shoot, error) {
	shoot := new(gardencorev1beta1.Shoot)
	err := k8sruntime.DefaultUnstructuredConverter.FromUnstructured(shootUnstructured.Object, shoot)
	if err != nil {
		return nil, err
	}
	return shoot, nil
}

func convertRuntimeObjToSecret(unstructuredSecret *unstructured.Unstructured) (*corev1.Secret, error) {
	secret := new(corev1.Secret)
	err := k8sruntime.DefaultUnstructuredConverter.FromUnstructured(unstructuredSecret.Object, secret)
	if err != nil {
		return nil, err
	}
	return secret, nil
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

func getDynamicClientForShoot(kubeconfig string) (dynamic.Interface, error) {
	restClientConfig, err := clientcmd.RESTConfigFromKubeConfig([]byte(kubeconfig))
	if err != nil {
		return nil, err
	}
	dynamicClient, err := dynamic.NewForConfig(restClientConfig)
	return dynamicClient, nil
}

//func getErrResult(err error, msg, shootName string) Result {
//	return Result{
//		Output: EventStream{
//			ShootName: shootName,
//		},
//		Err: errors.Wrapf(err, msg),
//	}
//}

func getSecretForShoot(ctx context.Context, shootName string, secretClient dynamic.ResourceInterface) (*corev1.Secret, error) {
	shootKubeconfigName := fmt.Sprintf("%s.kubeconfig", shootName)
	unstructuredSecret, err := secretClient.Get(ctx, shootKubeconfigName, metaV1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return convertRuntimeObjToSecret(unstructuredSecret)
}

func getNodes(ctx context.Context, client dynamic.Interface) (*corev1.NodeList, error) {

	nodeClient := client.Resource(nodeGVR)
	nodesUnstructured, err := nodeClient.List(ctx, metaV1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return convertRuntimeListToNodeList(nodesUnstructured)
}

func getShoot(ctx context.Context, shootName string, gardenerClient dynamic.ResourceInterface) (*gardenerv1beta1.Shoot, error) {
	unstructuredShoot, err := gardenerClient.Get(ctx, shootName, metaV1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return convertRuntimeObjToShoot(unstructuredShoot)
}
