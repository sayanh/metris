package process

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/kyma-incubator/metris/env"

	metristesting "github.com/kyma-incubator/metris/pkg/testing"

	"github.com/onsi/gomega"
)

const (
	timeout            = 10 * time.Second
	expectedPathPrefix = "/runtimes"
	providersFilePath  = "../testing/fixtures/static_providers.json"
)

func TestGetRuntimes(t *testing.T) {

	g := gomega.NewGomegaWithT(t)

	providersInfo, err := metristesting.LoadProvidersFromFile(providersFilePath)
	g.Expect(err).Should(gomega.BeNil())

	config := &env.Config{PublicCloudSpecs: string(providersInfo)}
	expectedProviders, err := LoadPublicCloudSpecs(config)
	g.Expect(err).Should(gomega.BeNil())

	getRuntimesHandler := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		// Health endpoint
		if req.URL.Path == "/health" {
			rw.WriteHeader(http.StatusOK)
			return
		}

		// Success endpoint
		if req.URL.Path == expectedPathPrefix {
			_, err := rw.Write(providersInfo)
			g.Expect(err).Should(gomega.BeNil())
			rw.WriteHeader(http.StatusOK)
			return
		}
	})

	// Start a local test HTTP server
	server := httptest.NewServer(getRuntimesHandler)
	// Close the server when test finishes
	defer server.Close()

	// Wait until server is ready
	g.Eventually(func() int {
		// Ignoring error is ok as it goes for retry for non-200 cases
		healthResp, _ := http.Get(fmt.Sprintf("%s/health", server.URL))
		return healthResp.StatusCode
	}, timeout).Should(gomega.Equal(http.StatusOK))

	kebURL, err := url.Parse(fmt.Sprintf("%s%s", server.URL, expectedPathPrefix))
	g.Expect(err).Should(gomega.BeNil())
	p := Process{
		KEBClient: &http.Client{
			Transport: http.DefaultTransport,
			Timeout:   2 * time.Second,
		},
		KEBReq: &http.Request{
			Method: http.MethodGet,
			URL:    kebURL,
		},
	}

	gotRuntimes, err := p.getRuntimes()
	g.Expect(err).Should(gomega.BeNil())

	g.Expect(gotRuntimes).To(gomega.Equal(expectedProviders))

}
