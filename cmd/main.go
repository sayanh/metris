package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/gorilla/mux"

	"github.com/kyma-incubator/metris/pkg/service"

	metrisprocess "github.com/kyma-incubator/metris/pkg/process"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"k8s.io/client-go/dynamic"

	gardenerv1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/kelseyhightower/envconfig"
	"github.com/kyma-incubator/metris/env"
	"github.com/kyma-incubator/metris/options"
	"github.com/kyma-incubator/metris/pkg/process"
	gocache "github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	log      = logrus.New()
	shootGVR = schema.GroupVersionResource{
		Version:  gardenerv1beta1.SchemeGroupVersion.Version,
		Group:    gardenerv1beta1.SchemeGroupVersion.Group,
		Resource: "shoots",
	}

	secretGVR = schema.GroupVersionResource{
		Version:  corev1.SchemeGroupVersion.Version,
		Group:    corev1.SchemeGroupVersion.Group,
		Resource: "secrets",
	}
)

func main() {
	opts := options.ParseArgs()
	log.Printf("Starting application with options: ", opts.String())

	cfg := new(env.Config)
	if err := envconfig.Process("", cfg); err != nil {
		log.Fatalf("Start handler failed with error: %s", err)
	}

	// Load public cloud specs
	publicCloudSpecs, err := LoadPublicCloudSpecs(cfg)
	if err != nil {
		log.Fatalf("failed to load public cloud specs: %v", err)
	}
	// Start a server for health check, metrics

	// Create dynamic client for gardener to get shoot and secret
	k8sConfig := GetGardenerKubeconfig(opts)
	client, err := k8sConfig.ClientConfig()
	if err != nil {
		log.Panicf("failed to generate client for k8s: %v", client)
	}
	restConfig := dynamic.ConfigFor(client)
	dyClient := dynamic.NewForConfigOrDie(restConfig)
	shootClient := dyClient.Resource(shootGVR).Namespace(opts.GardenerNamespace)
	secretClient := dyClient.Resource(secretGVR).Namespace(opts.GardenerNamespace)

	// Create an HTTP client to talk to KEB
	kebReq := &http.Request{
		Method: http.MethodGet,
		URL:    opts.KEBRuntimeEndpoint,
	}

	kebClient := http.DefaultClient
	resultChan := make(chan process.Result)
	cache := gocache.New(-1*time.Second, 0*time.Second)

	metrisProcess := metrisprocess.Process{
		KEBClient:      kebClient,
		KEBReq:         kebReq,
		GardenerClient: shootClient,
		SecretClient:   secretClient,
		Logger:         log,
		Providers:      publicCloudSpecs,
		Cache:          cache,
		ResultChan:     resultChan,
		CronInterval:   opts.CronInterval,
	}

	// Start after processing go-routine
	go metrisProcess.AfterProcess()

	router := mux.NewRouter()
	router.Path("/healthz").HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
	})
	router.Path("/metrics").Handler(promhttp.Handler())

	metrisSvr := service.Server{
		Addr:   fmt.Sprintf(":%d", opts.ListenAddr),
		Logger: log,
		Router: router,
	}

	go func() {
		// Start a server to cater to the metrics and healthz endpoints
		metrisSvr.Start()
	}()

	// Start a cron, which is a blocking call
	metrisProcess.RunCron()

	// add debug service.
	if opts.DebugPort > 0 {

		debugRouter := mux.NewRouter()
		// for security reason we always listen on localhost
		debugsvc := service.Server{
			Addr:   fmt.Sprintf("127.0.0.1:%d", opts.DebugPort),
			Logger: log,
			Router: debugRouter,
		}

		debugRouter.HandleFunc("/debug/pprof/", pprof.Index)
		debugRouter.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		debugRouter.HandleFunc("/debug/pprof/profile", pprof.Profile)
		debugRouter.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		debugRouter.HandleFunc("/debug/pprof/trace", pprof.Trace)
		debugRouter.Handle("/debug/pprof/block", pprof.Handler("block"))
		debugRouter.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
		debugRouter.Handle("/debug/pprof/heap", pprof.Handler("heap"))
		debugRouter.Handle("/debug/pprof/threadcreate", pprof.Handler("threadcreate"))
		go func() {
			debugsvc.Start()
		}()
	}

}

func LoadPublicCloudSpecs(cfg *env.Config) (*process.Providers, error) {
	if cfg.PublicCloudSpecs == "" {
		return nil, fmt.Errorf("public cloud specification is not configured")
	}
	publicCloudSpecs := new(process.Providers)
	err := json.Unmarshal([]byte(cfg.PublicCloudSpecs), publicCloudSpecs)
	if err != nil {
		return nil, err
	}
	return publicCloudSpecs, nil
}

func GetGardenerKubeconfig(opts *options.Options) clientcmd.ClientConfig {
	loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: opts.GardenerSecretPath}
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	return kubeConfig
}
