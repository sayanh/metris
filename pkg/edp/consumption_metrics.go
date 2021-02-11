package edp

type ConsumptionMetrics struct {
	//ResourceGroups []string `json:"resource_groups" validate:"required"`
	Compute    Compute    `json:"compute" validate:"required"`
	Networking Networking `json:"networking" validate:"required"`
}
type Networking struct {
	ProvisionedVnets int `json:"provisioned_vnets" validate:"numeric"`
	ProvisionedIPs   int `json:"provisioned_ips" validate:"numeric"`
}

type VMType struct {
	Name  string `json:"name" validate:"required"`
	Count int    `json:"count" validate:"numeric"`
}

type Compute struct {
	VMTypes            []VMType           `json:"vm_types" validate:"required"`
	ProvisionedCpus    int                `json:"provisioned_cpus" validate:"numeric"`
	ProvisionedRAMGb   float64            `json:"provisioned_ram_gb" validate:"numeric"`
	ProvisionedVolumes ProvisionedVolumes `json:"provisioned_volumes" validate:"required"`
}

type ProvisionedVolumes struct {
	SizeGbTotal   int `json:"size_gb_total" validate:"numeric"`
	Count         int `json:"count" validate:"numeric"`
	SizeGbRounded int `json:"size_gb_rounded" validate:"numeric"`
}
