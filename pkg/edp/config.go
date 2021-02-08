package edp

import "time"

type Config struct {
	URL               string        `envconfig:"EDP_URL" default:"https://input.yevents.io" required:"true"`
	Token             string        `envconfig:"EDP_TOKEN" required:"true"`
	Namespace         string        `envconfig:"EDP_NAMESPACE" default:"kyma-dev" required:"true"`
	DataStream        string        `envconfig:"EDP_DATASTREAM_NAME" default:"consumption-metrics" required:"true"`
	DataStreamVersion string        `envconfig:"EDP_DATASTREAM_VERSION" default:"1" required:"true"`
	DataStreamEnv     string        `envconfig:"EDP_DATASTREAM_ENV" default:"dev" required:"true"`
	Timeout           time.Duration `envconfig:"EDP_TIMEOUT" default:"30s"`
	//Buffer            int           `envconfig:"EDP_BUFFER" default:"100"`
	EventRetry int `envconfig:"EDP_RETRY" default:"5"`
}
