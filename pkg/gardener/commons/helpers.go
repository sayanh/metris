package commons

import (
	"github.com/kyma-incubator/metris/options"
	"k8s.io/client-go/tools/clientcmd"
)

func GetGardenerKubeconfig(opts *options.Options) clientcmd.ClientConfig {
	loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: opts.GardenerSecretPath}
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	return kubeConfig
}
