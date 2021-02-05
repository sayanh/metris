package process

import (
	"testing"

	metristesting "github.com/kyma-incubator/metris/pkg/testing"
	kebruntime "github.com/kyma-project/control-plane/components/kyma-environment-broker/common/runtime"
	"github.com/stretchr/testify/assert"
)

func TestFilterRuntimes(t *testing.T) {
	testCases := []struct {
		name             string
		inputRuntimes    kebruntime.RuntimesPage
		expectedRuntimes kebruntime.RuntimesPage
	}{
		{
			name: "filter out 1 RuntimesPage from 2",
			inputRuntimes: kebruntime.RuntimesPage{
				Data: []kebruntime.RuntimeDTO{
					metristesting.NewRuntimesDTO("foo1", metristesting.WithSucceededState),
					metristesting.NewRuntimesDTO("foo2", metristesting.WithFailedState),
				},
				Count:      2,
				TotalCount: 2,
			},
			expectedRuntimes: kebruntime.RuntimesPage{
				Data: []kebruntime.RuntimeDTO{
					metristesting.NewRuntimesDTO("foo1", metristesting.WithSucceededState),
				},
				Count:      1,
				TotalCount: 1,
			},
		},
		{
			name: "filter out 0 RuntimesPage from 2",
			inputRuntimes: kebruntime.RuntimesPage{
				Data: []kebruntime.RuntimeDTO{
					metristesting.NewRuntimesDTO("foo1", metristesting.WithFailedState),
					metristesting.NewRuntimesDTO("foo2", metristesting.WithFailedState),
				},
				Count:      2,
				TotalCount: 2,
			},
			expectedRuntimes: kebruntime.RuntimesPage{
				Data:       nil,
				Count:      0,
				TotalCount: 0,
			},
		},
		{
			name: "filter out 2 RuntimesPage from 2",
			inputRuntimes: kebruntime.RuntimesPage{
				Data: []kebruntime.RuntimeDTO{
					metristesting.NewRuntimesDTO("foo1", metristesting.WithSucceededState),
					metristesting.NewRuntimesDTO("foo2", metristesting.WithSucceededState),
				},
				Count:      2,
				TotalCount: 2,
			},
			expectedRuntimes: kebruntime.RuntimesPage{
				Data: []kebruntime.RuntimeDTO{
					metristesting.NewRuntimesDTO("foo1", metristesting.WithSucceededState),
					metristesting.NewRuntimesDTO("foo2", metristesting.WithSucceededState),
				},
				Count:      2,
				TotalCount: 2,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotRuntimesPage := filterRuntimes(tc.inputRuntimes)
			assert.Equal(t, tc.expectedRuntimes, *gotRuntimesPage, "filteredRuntimes should return only the ones with succeeded")
		})
	}
}
