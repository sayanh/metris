package options

import (
	"flag"
	"fmt"
	"log"
	"net/url"
	"time"

	"github.com/sirupsen/logrus"
)

type Options struct {
	GardenerSecretPath string
	GardenerNamespace  string
	KEBRuntimeEndpoint *url.URL
	CronInterval       time.Duration
	WorkerPoolSize     int
	DebugPort          int
	ListenAddr         int
	LogLevel           logrus.Level
}

func ParseArgs() *Options {
	gardenerSecretPath := flag.String("gardener-secret-path", "/gardener/kubeconfig", "The path to the secret which contains kubeconfig of the Gardener MPS cluster")
	gardenerNamespace := flag.String("gardener-namespace", "garden-kyma-dev", "The namespace in gardener cluster where information on Kyma clusters are")
	kebRuntimeEndpoint := flag.String("keb-runtime-endpoint", "http://kcp-kyma-environment-broker.kcp-system/runtimes", "The path to the secret which contains kubeconfig of the Gardener MPS cluster")
	cronInterval := flag.Duration("cron-interval", 60*time.Minute, "The duration of the interval between 2 executions of the process.")
	workerPoolSize := flag.Int("worker-pool-size", 1, "The path to the secret which contains kubeconfig of the Gardener MPS cluster")
	logLevelStr := flag.String("log-level", "debug", "The log-level of the application. E.g. fatal, error, info, debug etc.")
	listenAddr := flag.Int("listen-port", 8080, "The application starts server in this port to cater to the metrics and health endpoints.")
	debugPort := flag.Int("debug-port", 0, "The custom port to debug when needed.")
	flag.Parse()

	logLevel, err := logrus.ParseLevel(*logLevelStr)
	if err != nil {
		log.Fatalf("failed to parse level: %v", logLevel)
	}

	kebRuntimeURL, err := url.ParseRequestURI(*kebRuntimeEndpoint)
	if err != nil {
		log.Fatalf("failed to parse URL: %v", err)
	}

	return &Options{
		GardenerSecretPath: *gardenerSecretPath,
		GardenerNamespace:  *gardenerNamespace,
		KEBRuntimeEndpoint: kebRuntimeURL,
		CronInterval:       *cronInterval,
		WorkerPoolSize:     *workerPoolSize,
		DebugPort:          *debugPort,
		LogLevel:           logLevel,
		ListenAddr:         *listenAddr,
	}
}

func (o *Options) String() string {
	return fmt.Sprintf("--gardener-secret=%s --gardener-namespace=%s --keb-runtime-endpoint=%s "+
		"--cron-interval=%s --workerPoolSize: %d",
		o.GardenerSecretPath, o.GardenerNamespace, o.KEBRuntimeEndpoint,
		o.CronInterval.String(), o.WorkerPoolSize)
}
