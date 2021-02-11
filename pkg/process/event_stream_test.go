package process

import (
	"testing"

	"github.com/kyma-incubator/metris/env"
	"github.com/kyma-incubator/metris/pkg/edp"
	metristesting "github.com/kyma-incubator/metris/pkg/testing"

	"github.com/onsi/gomega"
)

func TestParse(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	providersData, err := metristesting.LoadFixtureFromFile(providersFile)
	g.Expect(err).Should(gomega.BeNil())
	config := &env.Config{PublicCloudSpecs: string(providersData)}
	providers, err := LoadPublicCloudSpecs(config)
	g.Expect(err).Should(gomega.BeNil())

	testCases := []struct {
		name            string
		input           Input
		providers       Providers
		expectedMetrics edp.ConsumptionMetrics
		expectedErr     bool
	}{
		{
			name: "with Azure, 2 vm types, 3 pvcs(5,10 and 20Gi) and 2 svcs(1 clusterIP and 1 LoadBalancer)",
			input: Input{
				shoot:    metristesting.GetShoot("testShoot", metristesting.WithAzureProviderAndStandard_D8_v3VMs),
				nodeList: metristesting.Get2Nodes(),
				pvcList:  metristesting.Get3PVCs(),
				svcList:  metristesting.Get2SvcsOfDiffTypes(),
			},
			providers: *providers,
			expectedMetrics: edp.ConsumptionMetrics{
				//ResourceGroups: nil,
				Compute: edp.Compute{
					VMTypes: []edp.VMType{{
						Name:  "standard_d8_v3",
						Count: 2,
					}},
					ProvisionedCpus:  16,
					ProvisionedRAMGb: 64,
					ProvisionedVolumes: edp.ProvisionedVolumes{
						SizeGbTotal:   417,
						Count:         5,
						SizeGbRounded: 448,
					},
				},
				Networking: edp.Networking{
					ProvisionedVnets: 1,
					ProvisionedIPs:   1,
				},
			},
		},
		{
			name: "with Azure with 3 vms and no pvc and svc",
			input: Input{
				shoot:    metristesting.GetShoot("testShoot", metristesting.WithAzureProviderAndStandard_D8_v3VMs),
				nodeList: metristesting.Get3NodesWithStandardD8v3VMType(),
			},
			providers: *providers,
			expectedMetrics: edp.ConsumptionMetrics{
				//ResourceGroups: nil,
				Compute: edp.Compute{
					VMTypes: []edp.VMType{{
						Name:  "standard_d8_v3",
						Count: 3,
					}},
					ProvisionedCpus:  24,
					ProvisionedRAMGb: 96,
					ProvisionedVolumes: edp.ProvisionedVolumes{
						SizeGbTotal:   600,
						Count:         3,
						SizeGbRounded: 608,
					},
				},
				Networking: edp.Networking{
					ProvisionedVnets: 1,
					ProvisionedIPs:   0,
				},
			},
		},
		{
			name: "with Azure and vm type missing from the list of vmtypes",
			input: Input{
				shoot:    metristesting.GetShoot("testShoot", metristesting.WithAzureProviderAndFooVMType),
				nodeList: metristesting.Get3NodesWithFooVMType(),
			},
			providers:   *providers,
			expectedErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotMetrics, err := tc.input.Parse(&tc.providers)
			if err == nil {
				g.Expect(err).Should(gomega.BeNil())
				g.Expect(*gotMetrics).To(gomega.Equal(tc.expectedMetrics))
				return
			}
			g.Expect(err).ShouldNot(gomega.BeNil())
			g.Expect(gotMetrics).Should(gomega.BeNil())
		})
	}
}
