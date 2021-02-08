package process

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"

	"k8s.io/client-go/util/retry"

	"github.com/pkg/errors"

	"github.com/kyma-incubator/metris/pkg/edp"
	kebruntime "github.com/kyma-project/control-plane/components/kyma-environment-broker/common/runtime"
	"github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/dynamic"
)

type Process struct {
	KEBClient      *http.Client
	KEBReq         *http.Request
	EDPClient      *edp.Client
	GardenerClient dynamic.ResourceInterface
	SecretClient   dynamic.ResourceInterface
	ResultChan     chan Result
	Logger         *logrus.Logger
	Cache          *cache.Cache
	Providers      *Providers
	CronInterval   time.Duration
}

type Result struct {
	Output EventStream
	Err    error
}

var CustomBackoff = wait.Backoff{
	Steps:    4,
	Duration: 10 * time.Second,
	Factor:   5.0,
	Jitter:   0.1,
}

func (p Process) getRuntimes() (*kebruntime.RuntimesPage, error) {

	var resp *http.Response
	var err error
	err = retry.OnError(CustomBackoff, func(err error) bool {
		if err != nil {
			return true
		}
		return false
	}, func() (err error) {
		resp, err = p.KEBClient.Do(p.KEBReq)
		if err != nil {
			p.Logger.Warnf("will be retried: failed while getting runtimes from KEB: %v", err)
		}
		return
	})

	if err != nil {
		p.Logger.Errorf("failed to get runtimes from KEB: %v", err)
		return nil, errors.Wrapf(err, "failed to get runtimes from KEB")
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		p.Logger.Errorf("failed to read body: %v", err)
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	runtimesPage := new(kebruntime.RuntimesPage)
	if err := json.Unmarshal(body, runtimesPage); err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal runtimes response")
	}
	return runtimesPage, nil
}

func (p Process) scrapeGardenerCluster(wg *sync.WaitGroup, shootName string) {
	defer wg.Done()
	ctx := context.Background()

	// Get shoot CR
	shoot, err := getShoot(ctx, shootName, p.GardenerClient)
	if err != nil {
		p.ResultChan <- getErrResult(err, "failed to get shoot", shootName)
		return
	}
	tenant := shoot.Labels["subaccount"]

	// Get shoot kubeconfig secret
	secret, err := getSecretForShoot(ctx, shootName, p.SecretClient)
	if err != nil {
		p.ResultChan <- getErrResult(err, "failed to get secret for shoot: %v", shootName)
		return
	}

	// Get nodes
	nodeClient, err := getDynamicClientForShoot(string(secret.Data["kubeconfig"]))
	if err != nil {
		p.ResultChan <- getErrResult(err, "failed to generate dynamic client for nodes from shoot: %s", shootName)
		return
	}
	nodes, err := getNodes(ctx, nodeClient)
	if err != nil {
		p.ResultChan <- getErrResult(err, "failed to get nodes list from shoot: %s", shootName)
		return
	}

	p.Logger.Debugf("nodes count: %v", len(nodes.Items))
	if len(nodes.Items) == 0 {
		p.ResultChan <- getErrResult(fmt.Errorf("number of nodes returned is 0, hence cannot process"), "", shootName)
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
	eventStream := input.Parse(shoot.Name, tenant, p.Providers)
	p.ResultChan <- Result{
		Output: eventStream,
		Err:    nil,
	}
	p.Logger.Debugf("---- end of processing --- for shoot: %s", shootName)
}

func (p Process) getOldMetric(result Result) ([]byte, error) {

	var oldEventStreamData []byte
	var err error
	oldEventStream, found := p.Cache.Get(result.Output.ShootName)
	if !found {
		notFoundErr := fmt.Errorf("failed to get an old event stream for shoot: %s", result.Output.ShootName)
		p.Logger.Error(notFoundErr)
		return []byte{}, notFoundErr
	}
	if oldData, ok := oldEventStream.(edp.ConsumptionMetrics); ok {
		oldEventStreamData, err = json.Marshal(oldData)
		if err != nil {
			return []byte{}, err
		}
	}
	return oldEventStreamData, nil
}

func (p Process) addDataToCache(result Result) error {
	alreadyExistsErr := fmt.Errorf("Item %s already exists", result.Output.ShootName)
	err := p.Cache.Add(result.Output.ShootName, result.Output.Metric, 60*time.Minute)
	if err != nil {
		if err.Error() == alreadyExistsErr.Error() {
			p.Cache.Delete(result.Output.ShootName)
			err := p.Cache.Add(result.Output.ShootName, result.Output.Metric, 60*time.Minute)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (p Process) afterProcess() {

	for result := range p.ResultChan {
		var dataToBeSent []byte
		var err error
		if result.Err != nil {
			// Failed to generate metrics hence send the previous data from the store(if any)
			p.Logger.Errorf("processing failed for shoot: %s err: %v", result.Output.ShootName, result.Err)
			dataToBeSent, err = p.getOldMetric(result)
			if err != nil {
				// Nothing to do
				continue
			}
			// Log the old metric which will sent to EDP
			p.Logger.Errorf("sending an old eventstream for shoot: %s, %s", result.Output.ShootName, string(dataToBeSent))
		} else {
			// When metric was successfully generated
			dataToBeSent, err = json.Marshal(result.Output.Metric)
			if err != nil {
				dataToBeSent, err = p.getOldMetric(result)
				if err != nil {
					// Nothing to do
					continue
				}
			}
		}

		tenant := result.Output.Tenant
		// retry may make sense
		edpRequest, err := p.EDPClient.NewRequest(tenant, dataToBeSent)
		if err != nil {
			p.Logger.Errorf("failed to create a new event stream data request: %+v for shoot: %s", err, result.Output.ShootName)
			continue
		}
		resp, err := p.EDPClient.Send(edpRequest)
		if err != nil {
			p.Logger.Errorf("failed to send event stream data: %+v for shoot: %s", err, result.Output.ShootName)
			continue
		}
		if !isSuccess(resp.StatusCode) {
			p.Logger.Errorf("failed to send event stream data: %+v for shoot: %s, with HTTP status code: %d", result.Err, result.Output.ShootName, resp.StatusCode)
			continue
		}

		p.Logger.Infof("successfully sent event stream for shoot: %s", result.Output.ShootName)
		// If no err then save it in the store
		if result.Err == nil {
			err = p.addDataToCache(result)
			if err != nil {
				p.Logger.Errorf("failed to save data to cache: %v for shoot: %s", err, result.Output.ShootName)
				return
			}
		}
		p.Logger.Debugf("successfully saved metric for shoot %s, %s", result.Output.ShootName, string(dataToBeSent))
	}
}

func (p Process) AfterProcess() {
	p.afterProcess()
}

func (p Process) runCron() {
	for {
		runtimesPage, err := p.getRuntimes()
		if err != nil {
			p.Logger.Errorf("failed to get runtimes from KEB: %v", err)
			continue
		}
		p.Logger.Debugf("num of runtimes are: %d", runtimesPage.Count)
		filteredRuntimes := filterRuntimes(*runtimesPage)
		p.Logger.Debugf("after filtration: num of runtimes are: %d", filteredRuntimes.Count)

		// Spawn workers to do the job(data processing and take it to EDP) and communicate back to the main routine once done
		var wg sync.WaitGroup
		wg.Add(len(filteredRuntimes.Data))
		for _, skrRuntime := range filteredRuntimes.Data {
			go p.scrapeGardenerCluster(&wg, skrRuntime.ShootName)
		}
		wg.Wait()

		p.Logger.Infof("### next execution will start in %v.......", p.CronInterval)
		time.Sleep(p.CronInterval)
	}
}

func (p Process) RunCron() {
	p.runCron()
}
