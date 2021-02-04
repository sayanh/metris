package edp

import (
	"bytes"
	"fmt"
	"net/http"
	"time"

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

func NewClient(timeout time.Duration, config *Config) *Client {
	httpClient := &http.Client{
		Transport: http.DefaultTransport,
		Timeout:   timeout,
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

	resp, err := eClient.HttpClient.Do(req)
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
