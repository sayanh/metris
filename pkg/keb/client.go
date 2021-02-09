package keb

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/pkg/errors"

	"k8s.io/apimachinery/pkg/util/wait"

	kebruntime "github.com/kyma-project/control-plane/components/kyma-environment-broker/common/runtime"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/util/retry"
)

type Client struct {
	HTTPClient *http.Client
	Logger     *logrus.Logger
	Config     *Config
}

func NewClient(config *Config, logger *logrus.Logger) *Client {
	kebHTTPClient := &http.Client{
		Transport: http.DefaultTransport,
		Timeout:   config.Timeout,
	}
	return &Client{
		HTTPClient: kebHTTPClient,
		Logger:     logger,
		Config:     config,
	}
}

func (c Client) NewRequest() (*http.Request, error) {
	c.Logger.Infof("###### config: %v", c.Config)
	kebURL, err := url.ParseRequestURI(c.Config.URL)
	if err != nil {
		return nil, err
	}
	req := &http.Request{
		Method: http.MethodGet,
		URL:    kebURL,
	}
	return req, nil
}

func (c Client) GetRuntimes(req *http.Request) (*kebruntime.RuntimesPage, error) {
	c.Logger.Infof(" polling for runtimes")

	customBackoff := wait.Backoff{
		Steps:    c.Config.RetryCount,
		Duration: c.HTTPClient.Timeout,
		Factor:   5.0,
		Jitter:   0.1,
	}
	var resp *http.Response
	var err error
	err = retry.OnError(customBackoff, func(err error) bool {
		if err != nil {
			return true
		}
		return false
	}, func() (err error) {
		resp, err = c.HTTPClient.Do(req)
		if err != nil {
			c.Logger.Warnf("will be retried: failed while getting runtimes from KEB: %v", err)
		}
		return
	})

	if err != nil {
		c.Logger.Errorf("failed to get runtimes from KEB: %v", err)
		return nil, errors.Wrapf(err, "failed to get runtimes from KEB")
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		c.Logger.Errorf("failed to read body: %v", err)
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	runtimesPage := new(kebruntime.RuntimesPage)
	if err := json.Unmarshal(body, runtimesPage); err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal runtimes response")
	}
	return runtimesPage, nil
}
