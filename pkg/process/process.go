package process

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/kyma-incubator/metris/pkg/keb"

	gardenerv1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"

	corev1 "k8s.io/api/core/v1"

	"k8s.io/client-go/util/workqueue"

	"github.com/pkg/errors"

	"github.com/kyma-incubator/metris/pkg/edp"
	kebruntime "github.com/kyma-project/control-plane/components/kyma-environment-broker/common/runtime"
	"github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/dynamic"
)

type Process struct {
	KEBClient       *keb.Client
	EDPClient       *edp.Client
	Queue           workqueue.DelayingInterface
	GardenerClient  dynamic.ResourceInterface
	SecretClient    dynamic.ResourceInterface
	Logger          *logrus.Logger
	Cache           *cache.Cache
	Providers       *Providers
	ScrapeInterval  time.Duration
	WorkersPoolSize int
}

type Result struct {
	Output EventStream
	Err    error
}

func (p Process) generateMetricFor(subAccountID string) (metric *edp.ConsumptionMetrics, kubeConfig string, shootName string, err error) {
	ctx := context.Background()

	obj, isFound := p.Cache.Get(subAccountID)
	if !isFound {
		err = fmt.Errorf("subAccountID was not found in cache")
		return
	}
	var engineInfo EngineInfo
	var ok bool
	if engineInfo, ok = obj.(EngineInfo); !ok {
		err = fmt.Errorf("bad item from cache")
		return
	}

	p.Logger.Infof("engine Info found: %+v", engineInfo)

	shootName = engineInfo.shootName

	if engineInfo.kubeConfig == "" {
		// Get shoot kubeconfig secret
		var secret *corev1.Secret
		secret, err = getSecretForShoot(ctx, shootName, p.SecretClient)
		if err != nil {
			return
		}
		engineInfo.kubeConfig = string(secret.Data["kubeconfig"])
	}

	// Get shoot CR
	var shoot *gardenerv1beta1.Shoot
	shoot, err = getShoot(ctx, shootName, p.GardenerClient)
	if err != nil {
		return
	}

	// Get nodes dynamic client
	var nodeClient dynamic.Interface
	nodeClient, err = getDynamicClientForShoot(engineInfo.kubeConfig)
	if err != nil {
		return
	}

	// Get nodes
	var nodes *corev1.NodeList
	nodes, err = getNodes(ctx, nodeClient)
	if err != nil {
		return
	}

	p.Logger.Debugf("nodes count: %v", len(nodes.Items))
	if len(nodes.Items) == 0 {
		err = fmt.Errorf("no nodes to process")
		return
	}

	//pvcClient
	//svcClient

	// Create input
	input := Input{
		nodes: nodes,
		shoot: shoot,
	}
	// Parse information and generate event stream
	defer func() {
		p.Logger.Debugf("---- end of processing --- for shoot: %s", shootName)
	}()
	metric, err = input.Parse(p.Providers)

	return
}

func (p Process) getOldMetric(subAccountID string) ([]byte, error) {
	var oldMetricData []byte
	var err error
	oldEngineInfoObj, found := p.Cache.Get(subAccountID)
	if !found {
		notFoundErr := fmt.Errorf("subAccountID: %s not found", subAccountID)
		p.Logger.Error(notFoundErr)
		return []byte{}, notFoundErr
	}

	if oldEngineInfo, ok := oldEngineInfoObj.(EngineInfo); ok {
		if oldEngineInfo.metric == nil {
			notFoundErr := fmt.Errorf("old metrics for subAccountID: %s not found", subAccountID)
			p.Logger.Error(notFoundErr)
			return []byte{}, notFoundErr
		}
		oldMetricData, err = json.Marshal(*oldEngineInfo.metric)
		if err != nil {
			return []byte{}, err
		}
	}
	return oldMetricData, nil
}

// pollKEBForRuntimes polls KEB for runtimes information
func (p Process) pollKEBForRuntimes() {
	kebReq, err := p.KEBClient.NewRequest()
	if err != nil {
		p.Logger.Fatalf("failed to create a new request for KEB: %v", err)
	}
	for {
		runtimesPage, err := p.KEBClient.GetRuntimes(kebReq)
		if err != nil {
			p.Logger.Errorf("failed to get runtimes from KEB: %v", err)
			time.Sleep(p.KEBClient.Config.PollWaitDuration)
			continue
		}
		p.Logger.Debugf("num of runtimes are: %d", runtimesPage.Count)
		p.populateCacheAndQueue(runtimesPage)
		// TODO remove me "TESTING PURPOSES"
		_ = p.Cache.Add("39ba9a66-2c1a-4fe4-a28e-6e5db434084e", EngineInfo{
			subAccountID: "39ba9a66-2c1a-4fe4-a28e-6e5db434084e",
			shootName:    "c-69e1cca",
			kubeConfig:   "",
			metric:       nil,
		}, cache.NoExpiration)
		// ------------------------------------
		p.Logger.Infof("waiting to poll KEB again after %v....", p.KEBClient.Config.PollWaitDuration)
		time.Sleep(p.KEBClient.Config.PollWaitDuration)
	}
}

// Start runs the whole of collection and sending metrics
func (p Process) Start() {

	var wg sync.WaitGroup
	go func() {
		p.pollKEBForRuntimes()
	}()

	for i := 0; i < p.WorkersPoolSize; i++ {
		go func() {
			defer wg.Done()
			p.Execute()
			p.Logger.Infof("########  Worker exits #######")
		}()
	}
	wg.Wait()
}

// Execute is executed by each worker to process an entry from the queue
func (p Process) Execute() {

	for {
		var payload, oldMetric []byte
		// Pick up a subAccountID to process from queue
		subAccountIDObj, isShuttingDown := p.Queue.Get()
		if isShuttingDown {
			//p.Cleanup()
			return
		}
		subAccountID := fmt.Sprintf("%v", subAccountIDObj)

		if subAccountID == "" {
			p.Logger.Warn("cannot work with empty subAccountID")
			continue
		}

		p.Logger.Infof("Subaccid: %v is fetched from queue", subAccountIDObj)
		metric, kubeconfig, shootName, err := p.generateMetricFor(subAccountID)
		if err != nil {
			p.Logger.Errorf("failed to generate new metric for subaccountID: %v, err: %v", subAccountID, err)
			// Get old data
			oldMetric, err = p.getOldMetric(subAccountID)
			if err != nil {
				p.Logger.Errorf("failed to getOldMetric for subaccountID: %s, err: %v", subAccountID, err)
				// Nothing to do
				continue
			}
		}

		if len(oldMetric) == 0 {
			payload, err = json.Marshal(metric)
			if err != nil {
				// Get old data
				oldMetric, err = p.getOldMetric(subAccountID)
				if err != nil {
					p.Logger.Errorf("failed to get old metric: %v", err)
					// Nothing to do
					continue
				}
			}
		} else {
			payload = oldMetric
		}

		// Note: EDP refers SubAccountID as tenant
		err = p.sendEventStreamToEDP(subAccountID, payload)
		if err != nil {
			// Nothing to do further hence continue
			p.Logger.Errorf("failed to send metric to EDP for subAccountID: %s, event-stream: %s", subAccountID, string(payload))
			continue
		}
		p.Logger.Infof("successfully sent event stream for subaccountID: %s, shoot: %s", subAccountID, shootName)

		if len(oldMetric) == 0 {
			newEngineInfo := EngineInfo{
				subAccountID: subAccountID,
				shootName:    shootName,
				kubeConfig:   kubeconfig,
				metric:       metric,
			}
			p.Cache.Set(newEngineInfo.subAccountID, newEngineInfo, cache.NoExpiration)
			p.Queue.AddAfter(subAccountID, p.ScrapeInterval)
			p.Logger.Infof("Length of queue: %d", p.Queue.Len())
			p.Logger.Debugf("successfully saved metric and requed for subAccountID %s, %s", newEngineInfo.subAccountID, string(payload))
		}
	}
}

func (p Process) sendEventStreamToEDP(tenant string, payload []byte) error {
	p.Logger.Infof("inside sendEventStreamToEDP: tenant: %s payload: %s", tenant, string(payload))
	edpRequest, err := p.EDPClient.NewRequest(tenant, payload)
	if err != nil {
		return errors.Wrapf(err, "failed to create a new request for EDP")
	}

	resp, err := p.EDPClient.Send(edpRequest)
	if err != nil {
		return errors.Wrapf(err, "failed to send event-stream to EDP")
	}

	if !isSuccess(resp.StatusCode) {
		return fmt.Errorf("failed to send event-stream to EDP as it returned HTTP: %d", resp.StatusCode)
	}
	return nil
}

type EngineInfo struct {
	subAccountID string
	shootName    string
	kubeConfig   string
	metric       *edp.ConsumptionMetrics
}

func isClusterTrackable(runtime *kebruntime.RuntimeDTO) bool {
	if runtime.Status.Provisioning != nil &&
		runtime.Status.Provisioning.State == "succeeded" &&
		runtime.Status.Deprovisioning == nil {
		return true
	}
	return false
}

// populateCacheAndQueue populates Cache and Queue with new runtimes and deletes the runtimes which should not be tracked
func (p Process) populateCacheAndQueue(runtimes *kebruntime.RuntimesPage) {

	for _, runtime := range runtimes.Data {
		_, isFound := p.Cache.Get(runtime.SubAccountID)
		if isClusterTrackable(&runtime) {
			if !isFound {
				engineInfo := EngineInfo{
					subAccountID: runtime.SubAccountID,
					shootName:    runtime.ShootName,
					kubeConfig:   "",
					metric:       nil,
				}
				if runtime.SubAccountID != "" {
					err := p.Cache.Add(runtime.SubAccountID, engineInfo, cache.NoExpiration)
					if err != nil {
						p.Logger.Errorf("failed to add subAccountID: %v to cache hence skipping queueing it", err)
						continue
					}
					p.Queue.Add(runtime.SubAccountID)
					p.Logger.Infof("Queued and added to cache: %v", runtime.SubAccountID)
				}
			}
		} else {
			if isFound {
				p.Logger.Infof("Deleting subAccountID: %v", runtime.SubAccountID)
				p.Cache.Delete(runtime.SubAccountID)
			}
		}
	}
	p.Logger.Infof("length of the cache: %d", p.Cache.ItemCount())
}
