package edp

import (
	"bytes"
	"fmt"
	"net/http"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"

	"github.com/pkg/errors"

	"github.com/sirupsen/logrus"
)

type Client struct {
	HttpClient *http.Client
	Config     *Config
	Logger     *logrus.Logger
}

const (
	edpPathFormat   = "%s/namespaces/%s/dataStreams/%s/%s/dataTenants/%s/%s/events"
	contentType     = "application/json;charset=utf-8"
	userAgentMetris = "metris"
)

func NewClient(config *Config) *Client {
	httpClient := &http.Client{
		Transport: http.DefaultTransport,
		Timeout:   config.Timeout,
	}
	return &Client{
		HttpClient: httpClient,
		Logger:     logrus.New(),
		Config:     config,
	}
}

func (eClient Client) NewRequest(dataTenant string, eventData []byte) (*http.Request, error) {
	edpURL := fmt.Sprintf(edpPathFormat,
		eClient.Config.URL,
		eClient.Config.Namespace,
		eClient.Config.DataStream,
		eClient.Config.DataStreamVersion,
		dataTenant,
		eClient.Config.DataStreamEnv,
	)

	eClient.Logger.Debugf("sending event to '%s'", edpURL)
	req, err := http.NewRequest(http.MethodPost, edpURL, bytes.NewBuffer(eventData))
	if err != nil {
		return nil, fmt.Errorf("failed generate request for EDP, %d: %v", http.StatusBadRequest, err)
	}

	req.Header.Set("User-Agent", userAgentMetris)
	req.Header.Add("Content-Type", contentType)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", eClient.Config.Token))

	return req, nil
}

func (eClient Client) Send(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error
	// TODO make it configurable
	customBackoff := wait.Backoff{
		Steps:    eClient.Config.EventRetry,
		Duration: 5 * time.Second,
		Factor:   5.0,
		Jitter:   0.1,
	}
	err = retry.OnError(customBackoff, func(err error) bool {
		if err != nil {
			return true
		}
		return false
	}, func() (err error) {
		resp, err = eClient.HttpClient.Do(req)
		if err != nil {
			eClient.Logger.Warnf("will be retried: failed to send event stream to EDP: %v", err)
			eClient.Logger.Warnf("req: %v", req)
			return
		}

		if resp.StatusCode != http.StatusCreated {
			non2xxErr := fmt.Errorf("failed to send event stream as EDP returned HTTP: %d", resp.StatusCode)
			eClient.Logger.Warnf("will be retried: %v", non2xxErr)
			err = non2xxErr
		}
		return
	})

	if err != nil {
		failedErr := errors.Wrapf(err, "failed to POST event to EDP")
		eClient.Logger.Errorf("%v", failedErr)
		return nil, failedErr
	}

	defer func() {
		err := resp.Body.Close()
		if err != nil {
			eClient.Logger.Warn(err)
		}
	}()

	return resp, nil
}
