package main

import (
	"fmt"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/kyma-incubator/metris/pkg/edp"

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

const (
	metricsPath = "/metrics"
	healthzPath = "/healthz"
)

func main() {
	opts := options.ParseArgs()
	log.Printf("Starting application with options: ", opts.String())

	cfg := new(env.Config)
	if err := envconfig.Process("", cfg); err != nil {
		log.Fatalf("failed to load env config: %s", err)
	}

	edpConfig := new(edp.Config)
	if err := envconfig.Process("", edpConfig); err != nil {
		log.Fatalf("failed to load EDP config: %s", err)
	}
	// Load public cloud specs
	publicCloudSpecs, err := metrisprocess.LoadPublicCloudSpecs(cfg)
	if err != nil {
		log.Fatalf("failed to load public cloud specs: %v", err)
	}
	log.Infof("public cloud spec: %v", publicCloudSpecs)

	// Create dynamic client for gardener to get shoot and secret
	k8sConfig := GetGardenerKubeconfig(opts)
	client, err := k8sConfig.ClientConfig()
	if err != nil {
		log.Panicf("failed to generate client for k8s: %v", client)
	}
	restConfig := dynamic.ConfigFor(client)
	dynClient := dynamic.NewForConfigOrDie(restConfig)
	shootClient := dynClient.Resource(shootGVR).Namespace(opts.GardenerNamespace)
	secretClient := dynClient.Resource(secretGVR).Namespace(opts.GardenerNamespace)

	// Create an HTTP client to talk to KEB
	kebReq := &http.Request{
		Method: http.MethodGet,
		URL:    opts.KEBRuntimeEndpoint,
	}

	kebClient := http.DefaultClient
	resultChan := make(chan process.Result)
	// Creating cache with no expiration
	cache := gocache.New(gocache.NoExpiration, 0*time.Second)

	edpClient := edp.NewClient(edpConfig)

	metrisProcess := metrisprocess.Process{
		KEBClient:      kebClient,
		KEBReq:         kebReq,
		GardenerClient: shootClient,
		SecretClient:   secretClient,
		EDPClient:      edpClient,
		Logger:         log,
		Providers:      publicCloudSpecs,
		Cache:          cache,
		ResultChan:     resultChan,
		CronInterval:   opts.CronInterval,
	}

	// Start after processing go-routine
	go metrisProcess.AfterProcess()

	// Start a cron for scrapping gardener and shoot clusters
	go metrisProcess.RunCron()

	// add debug service.
	if opts.DebugPort > 0 {
		enableDebugging(opts.DebugPort)
	}
	// Start a server to cater to the metrics and healthz endpoints
	router := mux.NewRouter()
	router.Path(healthzPath).HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
	})
	router.Path(metricsPath).Handler(promhttp.Handler())

	metrisSvr := service.Server{
		Addr:   fmt.Sprintf(":%d", opts.ListenAddr),
		Logger: log,
		Router: router,
	}

	metrisSvr.Start()
}

func enableDebugging(debugPort int) {
	debugRouter := mux.NewRouter()
	// for security reason we always listen on localhost
	debugSvc := service.Server{
		Addr:   fmt.Sprintf("127.0.0.1:%d", debugPort),
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
		debugSvc.Start()
	}()
}

func GetGardenerKubeconfig(opts *options.Options) clientcmd.ClientConfig {
	loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: opts.GardenerSecretPath}
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	return kubeConfig
}
