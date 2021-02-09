package main

import (
	"fmt"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/kyma-incubator/metris/pkg/keb"

	"github.com/kyma-incubator/metris/pkg/edp"
	"k8s.io/client-go/util/workqueue"

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

	log.Infof("log level: %s", log.Level.String())

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

	// Create a client for KEB communication
	kebConfig := new(keb.Config)
	if err := envconfig.Process("", kebConfig); err != nil {
		log.Fatalf("failed to load KEB config: %s", err)
	}
	kebClient := keb.NewClient(kebConfig, log)
	log.Infof("keb config: %v", kebConfig)
	// Creating cache with no expiration
	cache := gocache.New(gocache.NoExpiration, 0*time.Second)

	// Creating EDP client
	edpConfig := new(edp.Config)
	if err := envconfig.Process("", edpConfig); err != nil {
		log.Fatalf("failed to load EDP config: %s", err)
	}
	edpClient := edp.NewClient(edpConfig)

	queue := workqueue.NewDelayingQueue()

	metrisProcess := metrisprocess.Process{
		KEBClient:       kebClient,
		GardenerClient:  shootClient,
		SecretClient:    secretClient,
		EDPClient:       edpClient,
		Logger:          log,
		Providers:       publicCloudSpecs,
		Cache:           cache,
		ScrapeInterval:  opts.ScrapeInterval,
		Queue:           queue,
		WorkersPoolSize: opts.WorkerPoolSize,
	}

	// Start execution
	go metrisProcess.Start()

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
