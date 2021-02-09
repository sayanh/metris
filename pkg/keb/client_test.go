package keb

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
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
		// Health endpoint
		if req.URL.Path == "/health" {
			rw.WriteHeader(http.StatusOK)
			return
		}

		// Success endpoint
		if req.URL.Path == expectedPathPrefix {
			_, err := rw.Write(runtimesResponse)
			g.Expect(err).Should(gomega.BeNil())
			rw.WriteHeader(http.StatusOK)
			return
		}
	})

	// Start a local test HTTP server
	server := httptest.NewServer(getRuntimesHandler)
	// Close the server when test finishes
	defer server.Close()

	// Wait until test server is ready
	g.Eventually(func() int {
		// Ignoring error is ok as it goes for retry for non-200 cases
		healthResp, _ := http.Get(fmt.Sprintf("%s/health", server.URL))
		return healthResp.StatusCode
	}, timeout).Should(gomega.Equal(http.StatusOK))

	kebURL, err := url.Parse(fmt.Sprintf("%s%s", server.URL, expectedPathPrefix))
	g.Expect(err).Should(gomega.BeNil())
	kebClient := Client{
		HTTPClient:       http.DefaultClient,
		PollWaitDuration: 10 * time.Minute,
		Logger:           &logrus.Logger{},
		Request: &http.Request{
			Method: http.MethodGet,
			URL:    kebURL,
		},
	}

	gotRuntimes, err := kebClient.GetRuntimes()
	g.Expect(err).Should(gomega.BeNil())
	g.Expect(gotRuntimes).To(gomega.Equal(expectedRuntimes))
}
