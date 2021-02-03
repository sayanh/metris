package edp

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"
)

type Client struct {
	httpClient *http.Client
	url        *url.URL
}

func (eClient Client) NewClient(url *url.URL, timeout time.Duration) *Client {
	httpClient := http.Client{
		Transport:     http.DefaultTransport,
		CheckRedirect: nil,
		Jar:           nil,
		Timeout:       timeout,
	}
	return &Client{
		httpClient: &httpClient,
		url:        url,
	}
}

func (eClient Client) newRequest() *http.Request {

	return &http.Request{
		Method: http.MethodPost,
		URL:    eClient.url,
	}
}

func (eClient Client) Send(data []byte) (*http.Response, error) {
	req := eClient.newRequest()
	req.Body = ioutil.NopCloser(bytes.NewReader([]byte(data)))
	return eClient.httpClient.Do(req)
}
