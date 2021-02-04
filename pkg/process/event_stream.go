package process

import (
	"strings"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/kyma-incubator/metris/pkg/edp"
	corev1 "k8s.io/api/core/v1"
)

const (
	nodeLabel = "beta.kubernetes.io/instance-type"
)

type EventStream struct {
	ShootName string
	Metric    edp.ConsumptionMetrics
	Tenant    string
}

type Input struct {
	shoot *gardencorev1beta1.Shoot
	nodes *corev1.NodeList
}

type NodeInfo struct {
	cpu    int
	memory int
}

func (inp Input) Parse(shootName, tenant string, providers *Providers) EventStream {

	metric := new(edp.ConsumptionMetrics)
	provisionedCPUs := 0
	provisionedMemory := 0.0
	providerType := inp.shoot.Spec.Provider.Type
	vmTypes := make(map[string]int)
	for _, node := range inp.nodes.Items {
		nodeType := node.Labels[nodeLabel]
		nodeType = strings.ToLower(nodeType)
		vmFeatures := providers.GetFeatures(providerType, nodeType)
		provisionedCPUs += vmFeatures.CpuCores
		provisionedMemory += vmFeatures.Memory
		vmTypes[nodeType] += 1
	}
	metric.Compute.ProvisionedCpus = provisionedCPUs
	metric.Compute.ProvisionedRAMGb = provisionedMemory

	for vmType, count := range vmTypes {
		metric.Compute.VMTypes = append(metric.Compute.VMTypes, edp.VMType{
			Name:  vmType,
			Count: count,
		})
	}

	eventStream := EventStream{
		ShootName: shootName,
		Metric:    *metric,
		Tenant:    tenant,
	}
	return eventStream
}
