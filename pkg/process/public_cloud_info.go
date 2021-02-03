package process

type Providers struct {
	Data map[string]Provider `json:"data"`
}
type Provider struct {
	Specs Specs `json:"specs"`
}

type Specs struct {
	Vms map[string]Vm `json:"vms"`
}

type Vm struct {
	Features Features `json:"features"`
}

type Features struct {
	CpuCores int     `json:"cpu_cores"`
	Memory   float64 `json:"memory"`
	Storage  int     `json:"storage"`
	MaxNICs  int     `json:"max_nics"`
}

func (cp Providers) GetFeatures(providerName, vmName string) Features {
	provider := cp.Data[providerName]
	features := provider.Specs.Vms[vmName].Features
	return features
}
