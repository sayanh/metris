package keb

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/sirupsen/logrus"

	metristesting "github.com/kyma-incubator/metris/pkg/testing"
	"github.com/kyma-project/control-plane/components/kyma-environment-broker/common/runtime"
	"github.com/onsi/gomega"
)

const (
	timeout                    = 5 * time.Second
	expectedPathPrefix         = "/runtimes"
	kebRuntimeResponseFilePath = "../testing/fixtures/runtimes_response.json"
)

func TestGetRuntimes(t *testing.T) {

	g := gomega.NewGomegaWithT(t)

	runtimesResponse, err := metristesting.LoadFixtureFromFile(kebRuntimeResponseFilePath)
	g.Expect(err).Should(gomega.BeNil())
	expectedRuntimes := new(runtime.RuntimesPage)
	err = json.Unmarshal(runtimesResponse, expectedRuntimes)
	g.Expect(err).Should(gomega.BeNil())

	getRuntimesHandler := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {

		// Success endpoint
		if req.URL.Path == expectedPathPrefix {
			_, err := rw.Write(runtimesResponse)
			g.Expect(err).Should(gomega.BeNil())
			rw.WriteHeader(http.StatusOK)
			return
		}
	})

	// Start a local test HTTP server
	srv := metristesting.StartTestServer(expectedPathPrefix, getRuntimesHandler, g)

	// Wait until test server is ready
	g.Eventually(func() int {
		// Ignoring error is ok as it goes for retry for non-200 cases
		healthResp, err := http.Get(fmt.Sprintf("%s/health", srv.URL))
		g.Expect(err).Should(gomega.BeNil())

		return healthResp.StatusCode
	}, timeout).Should(gomega.Equal(http.StatusOK))

	kebURL := fmt.Sprintf("%s%s", srv.URL, expectedPathPrefix)

	config := &Config{
		URL:              kebURL,
		Timeout:          3 * time.Second,
		RetryCount:       1,
		PollWaitDuration: 10 * time.Minute,
	}
	kebClient := Client{
		HTTPClient: http.DefaultClient,
		Logger:     &logrus.Logger{},
		Config:     config,
	}

	req, err := kebClient.NewRequest()
	g.Expect(err).Should(gomega.BeNil())

	gotRuntimes, err := kebClient.GetRuntimes(req)
	g.Expect(err).Should(gomega.BeNil())
	g.Expect(gotRuntimes).To(gomega.Equal(expectedRuntimes))
}
