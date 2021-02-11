package process

import (
	"fmt"
	"math"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"

	"k8s.io/apimachinery/pkg/runtime"

	gardenerazurev1alpha1 "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/kyma-incubator/metris/pkg/edp"
	corev1 "k8s.io/api/core/v1"
)

const (
	nodeInstanceTypeLabel = "node.kubernetes.io/instance-type"
	// storageRoundingFactor rounds of storage to 32. E.g. 17 -> 32, 33 -> 64
	storageRoundingFactor = 32

	Azure = "azure"
)

type EventStream struct {
	Metric     edp.ConsumptionMetrics
	KubeConfig string
}

type Input struct {
	shoot    *gardencorev1beta1.Shoot
	nodeList *corev1.NodeList
	pvcList  *corev1.PersistentVolumeClaimList
	svcList  *corev1.ServiceList
}

type NodeInfo struct {
	cpu    int
	memory int
}

func (inp Input) Parse(providers *Providers) (*edp.ConsumptionMetrics, error) {

	if inp.nodeList == nil {
		return nil, fmt.Errorf("no nodes data to compute metrics on")
	}
	if inp.shoot == nil {
		return nil, fmt.Errorf("no shoot data to compute metrics on")
	}

	metric := new(edp.ConsumptionMetrics)
	provisionedCPUs := 0
	provisionedMemory := 0.0
	providerType := inp.shoot.Spec.Provider.Type
	vmTypes := make(map[string]int)

	nodeStorage := 0
	pvcStorage := 0
	volumeCount := 0
	vnets := 0

	for _, node := range inp.nodeList.Items {
		nodeType := node.Labels[nodeInstanceTypeLabel]
		nodeType = strings.ToLower(nodeType)

		// Calculate CPU and Memory
		vmFeatures := providers.GetFeatures(providerType, nodeType)
		if vmFeatures == nil {
			return nil, fmt.Errorf("providerType : %s and nodeType: %s does not exist in the map", providerType, nodeType)
		}
		provisionedCPUs += vmFeatures.CpuCores
		provisionedMemory += vmFeatures.Memory
		vmTypes[nodeType] += 1

		// Calculate node storage
		nodeStorage += vmFeatures.Storage
		volumeCount += 1

	}

	if inp.pvcList != nil {
		// Calculate storage from PVCs
		for _, pvc := range inp.pvcList.Items {
			pvcStorage += pvc.Status.Capacity.Storage().Size()
			volumeCount += 1
		}
	}

	provisionedIPs := 0
	if inp.svcList != nil {
		// Calculate network related information
		for _, svc := range inp.svcList.Items {
			if svc.Spec.Type == "LoadBalancer" {
				provisionedIPs += 1
			}
		}
	}

	// Calculate vnets
	if inp.shoot.Spec.Provider.InfrastructureConfig != nil {
		rawExtension := *inp.shoot.Spec.Provider.InfrastructureConfig
		switch inp.shoot.Spec.Provider.Type {

		// Raw extensions varies based on the provider type
		case Azure:
			decoder := serializer.NewCodecFactory(scheme.Scheme).UniversalDecoder()
			infraConfig := &gardenerazurev1alpha1.InfrastructureConfig{}
			err := runtime.DecodeInto(decoder, rawExtension.Raw, infraConfig)
			if err != nil {
				return nil, err
			}
			if infraConfig.Networks.VNet.CIDR != nil {
				vnets += 1
			}
		default:
			return nil, fmt.Errorf("provider: %s does not match in the system", inp.shoot.Spec.Provider.Type)
		}
	}
	metric.Compute.ProvisionedCpus = provisionedCPUs
	metric.Compute.ProvisionedRAMGb = provisionedMemory

	totalActualStorage := nodeStorage + pvcStorage
	metric.Compute.ProvisionedVolumes.SizeGbTotal = totalActualStorage
	metric.Compute.ProvisionedVolumes.SizeGbRounded = getVolumeRoundedToFactor(totalActualStorage)
	metric.Compute.ProvisionedVolumes.Count = volumeCount

	metric.Networking.ProvisionedIPs = provisionedIPs
	metric.Networking.ProvisionedVnets = vnets

	for vmType, count := range vmTypes {
		metric.Compute.VMTypes = append(metric.Compute.VMTypes, edp.VMType{
			Name:  vmType,
			Count: count,
		})
	}

	return metric, nil
}

func getVolumeRoundedToFactor(size int) int {
	return int(math.Ceil(float64(size)/storageRoundingFactor) * storageRoundingFactor)
}
