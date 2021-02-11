package process

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/kyma-incubator/metris/pkg/keb"

	gardenerv1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	metriscache "github.com/kyma-incubator/metris/pkg/cache"
	gardenersecret "github.com/kyma-incubator/metris/pkg/gardener/secret"
	gardenershoot "github.com/kyma-incubator/metris/pkg/gardener/shoot"
	skrnode "github.com/kyma-incubator/metris/pkg/skr/node"
	skrpvc "github.com/kyma-incubator/metris/pkg/skr/pvc"
	skrsvc "github.com/kyma-incubator/metris/pkg/skr/svc"

	corev1 "k8s.io/api/core/v1"

	"k8s.io/client-go/util/workqueue"

	"github.com/pkg/errors"

	"github.com/kyma-incubator/metris/pkg/edp"
	kebruntime "github.com/kyma-project/control-plane/components/kyma-environment-broker/common/runtime"
	"github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"
)

type Process struct {
	KEBClient       *keb.Client
	EDPClient       *edp.Client
	Queue           workqueue.DelayingInterface
	ShootClient     *gardenershoot.Client
	SecretClient    *gardenersecret.Client
	Cache           *cache.Cache
	Providers       *Providers
	ScrapeInterval  time.Duration
	WorkersPoolSize int
	Logger          *logrus.Logger
}

func (p Process) generateMetricFor(subAccountID string) (metric *edp.ConsumptionMetrics, kubeConfig string, shootName string, err error) {
	ctx := context.Background()
	var record metriscache.Record
	var ok bool

	obj, isFound := p.Cache.Get(subAccountID)
	if !isFound {
		err = fmt.Errorf("subAccountID was not found in cache")
		return
	}
	if record, ok = obj.(metriscache.Record); !ok {
		err = fmt.Errorf("bad item from cache")
		return
	}
	p.Logger.Debugf("record found from cache: %+v", record)

	shootName = record.ShootName

	if record.KubeConfig == "" {
		// Get shoot kubeconfig secret
		var secret *corev1.Secret
		secret, err = p.SecretClient.Get(ctx, shootName)
		if err != nil {
			return
		}
		record.KubeConfig = string(secret.Data["kubeconfig"])
	}

	// Get shoot CR
	var shoot *gardenerv1beta1.Shoot
	shoot, err = p.ShootClient.Get(ctx, shootName)
	if err != nil {
		return
	}

	// Get nodes dynamic client
	nodesClient, err := skrnode.NewClient(record.KubeConfig)
	if err != nil {
		return
	}

	// Get nodes
	var nodes *corev1.NodeList
	nodes, err = nodesClient.List(ctx)
	if err != nil {
		return
	}

	p.Logger.Debugf("nodes count: %v", len(nodes.Items))
	if len(nodes.Items) == 0 {
		err = fmt.Errorf("no nodes to process")
		return
	}

	// Get PVCs
	pvcClient, err := skrpvc.NewClient(record.KubeConfig)
	var pvcList *corev1.PersistentVolumeClaimList
	pvcList, err = pvcClient.List(ctx)
	if err != nil {
		return
	}

	// Get Svcs
	var svcList *corev1.ServiceList
	svcClient, err := skrsvc.NewClient(record.KubeConfig)
	if err != nil {
		return
	}
	svcList, err = svcClient.List(ctx)
	if err != nil {
		return
	}

	// Create input
	input := Input{
		shoot:    shoot,
		nodeList: nodes,
		pvcList:  pvcList,
		svcList:  svcList,
	}
	// Parse information and generate event stream
	defer func() {
		p.Logger.Debugf("---- end of processing --- for shoot: %s", shootName)
	}()
	metric, err = input.Parse(p.Providers)

	return
}

// getOldMetric gets metric information for the given subAccountID which was saved in the cache earlier
func (p Process) getOldMetric(subAccountID string) ([]byte, error) {
	var oldMetricData []byte
	var err error
	oldRecordObj, found := p.Cache.Get(subAccountID)
	if !found {
		notFoundErr := fmt.Errorf("subAccountID: %s not found", subAccountID)
		p.Logger.Error(notFoundErr)
		return []byte{}, notFoundErr
	}

	if oldRecord, ok := oldRecordObj.(metriscache.Record); ok {
		if oldRecord.Metric == nil {
			notFoundErr := fmt.Errorf("old metrics for subAccountID: %s not found", subAccountID)
			p.Logger.Error(notFoundErr)
			return []byte{}, notFoundErr
		}
		oldMetricData, err = json.Marshal(*oldRecord.Metric)
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
		testSubAccID := "8720b2bb-ffff-45b0-8448-799c22b85ac0"
		testShootName := "c0983ec"
		_ = p.Cache.Add(testSubAccID, metriscache.Record{
			SubAccountID: testSubAccID,
			ShootName:    testShootName,
			KubeConfig:   "",
			Metric:       nil,
		}, cache.NoExpiration)
		p.Queue.Add(testSubAccID)
		// ------------------------------------

		p.Logger.Infof("length of the cache: %d", p.Cache.ItemCount())
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
			newRecord := metriscache.Record{
				SubAccountID: subAccountID,
				ShootName:    shootName,
				KubeConfig:   kubeconfig,
				Metric:       metric,
			}
			p.Cache.Set(newRecord.SubAccountID, newRecord, cache.NoExpiration)
			p.Queue.AddAfter(subAccountID, p.ScrapeInterval)
			p.Logger.Infof("Length of queue: %d", p.Queue.Len())
			p.Logger.Debugf("successfully saved metric and requed for subAccountID %s, %s", newRecord.SubAccountID, string(payload))
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

func isSuccess(status int) bool {
	if status >= http.StatusOK && status < http.StatusMultipleChoices {
		return true
	}
	return false
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
				record := metriscache.Record{
					SubAccountID: runtime.SubAccountID,
					ShootName:    runtime.ShootName,
					KubeConfig:   "",
					Metric:       nil,
				}
				if runtime.SubAccountID != "" {
					err := p.Cache.Add(runtime.SubAccountID, record, cache.NoExpiration)
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
}
