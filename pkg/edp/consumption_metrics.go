package edp

type ConsumptionMetrics struct {
	//ResourceGroups []string `json:"resource_groups" validate:"required"`
	Compute    Compute `json:"compute" validate:"required"`
	Networking struct {
		ProvisionedLoadbalancers int `json:"provisioned_loadbalancers" validate:"numeric"`
		ProvisionedVnets         int `json:"provisioned_vnets" validate:"numeric"`
		ProvisionedIPs           int `json:"provisioned_ips" validate:"numeric"`
	} `json:"networking" validate:"required"`
}

type VMType struct {
	Name  string `json:"name" validate:"required"`
	Count int    `json:"count" validate:"numeric"`
}

type Compute struct {
	VMTypes            []VMType `json:"vm_types" validate:"required"`
	ProvisionedCpus    int      `json:"provisioned_cpus" validate:"numeric"`
	ProvisionedRAMGb   float64  `json:"provisioned_ram_gb" validate:"numeric"`
	ProvisionedVolumes struct {
		SizeGbTotal   int `json:"size_gb_total" validate:"numeric"`
		Count         int `json:"count" validate:"numeric"`
		SizeGbRounded int `json:"size_gb_rounded" validate:"numeric"`
	} `json:"provisioned_volumes" validate:"required"`
}
