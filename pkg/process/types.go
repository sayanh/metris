package process

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"istio.io/pkg/log"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kebruntime "github.com/kyma-project/control-plane/components/kyma-environment-broker/common/runtime"
	"github.com/pkg/errors"

	"github.com/kyma-incubator/metris/pkg/edp"
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

func (p Process) GetKEBResponse() (*kebruntime.RuntimesPage, error) {

	p.Logger.Info("are we in GetKEBResponse")
	var resp *http.Response
	var err error
	// TODO Add a retry logic
	//retryErr := retry.OnError(wait.Backoff{
	//	Duration: 10 * time.Second,
	//	Factor:   0,
	//	Jitter:   0,
	//	Steps:    0,
	//	Cap:      0,
	//}, func(err error) bool {
	//	p.Logger.Infof("are we here in retriable")
	//	return true
	//}, func() error {
	//	p.Logger.Infof("are we here")
	//	resp, err = p.KEBClient.Do(p.KEBReq)
	//	if err != nil {
	//		return errors.Wrapf(err, "failed to get response from KEB")
	//	}
	//	if resp.StatusCode != http.StatusOK {
	//		return fmt.Errorf("keb responded with non 200: %d", resp.StatusCode)
	//	}
	//	return nil
	//})
	resp, retryErr := p.KEBClient.Do(p.KEBReq)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get response from KEB")
	}
	if retryErr != nil {
		p.Logger.Errorf("failed to retry to make a req to KEB: %v", err)
		return nil, retryErr
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
		return nil, err
	}

	return runtimesPage, retryErr
}

func (p Process) scrapeGardenerCluster(wg *sync.WaitGroup, shootName string) {
	defer wg.Done()
	ctx := context.Background()
	// Get shoot CR
	unstructuredShoot, err := p.GardenerClient.Get(ctx, shootName, metaV1.GetOptions{})
	if err != nil {
		p.ResultChan <- getErrResult(err, "failed to get shoots", shootName)
		return
	}
	shoot, err := convertRuntimeObjToShoot(unstructuredShoot)
	if err != nil {
		p.ResultChan <- getErrResult(err, "failed to convert unstructured to shoot", shootName)
		return
	}

	// Get shoot kubeconfig secret
	shootKubeconfigName := fmt.Sprintf("%s.kubeconfig", shootName)
	unstructuredSecret, err := p.SecretClient.Get(ctx, shootKubeconfigName, metaV1.GetOptions{})
	if err != nil {
		p.ResultChan <- getErrResult(err, "failed to get secrets", shootName)
		return
	}
	secret, err := convertRuntimeObjToSecret(unstructuredSecret)
	if err != nil {
		p.ResultChan <- getErrResult(err, "failed to convert unstructured to secret", shootName)
		return
	}

	shootKubeconfig := string(secret.Data["kubeconfig"])

	// Create a dynamicClient for nodes in Shoot cluster
	dynamicClientForShoot, err := getDynamicClientForShoot(shootKubeconfig)
	if err != nil {
		p.ResultChan <- getErrResult(err, "failed to convert unstructured list to node list", shootName)
		return
	}
	nodeClient := dynamicClientForShoot.Resource(nodeGVR)
	nodesUnstructured, err := nodeClient.List(ctx, metaV1.ListOptions{})
	if err != nil {
		p.ResultChan <- getErrResult(err, "failed to list nodes", shootName)
		return
	}
	p.Logger.Infof("unstructured nodes list: %v", *nodesUnstructured)

	// Reach shoot cluster
	nodes, err := convertRuntimeListToNodeList(nodesUnstructured)
	if err != nil {
		p.ResultChan <- getErrResult(err, "failed to convert unstructured list to node list", shootName)
		return
	}

	p.Logger.Infof("nodes : %v", *nodes)
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

	// Parse information
	eventStream := input.Parse(shoot.Name, p.Providers)

	p.ResultChan <- Result{
		Output: eventStream,
		Err:    nil,
	}
	result := fmt.Sprintf("processing done for: %s\n", shootName)
	log.Infof("---- end of processing --- : %v", result)
	// Send the data to EDP
	//p.EDPClient.Send()

	//resultChan <- Result{
	//	output: result,
	//	err:    nil,

}

type Result struct {
	Output EventStream
	Err    error
}

func (p Process) getOldMetric(result Result) ([]byte, error) {

	var oldEventStreamData []byte
	var err error
	oldEventStream, found := p.Cache.Get(result.Output.ShootName)
	if !found {
		notFoundErr := fmt.Errorf("failed to get old event stream for shoot: %s", result.Output.ShootName)
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

func (p Process) AfterProcess() {
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

		p.Logger.Infof("sending data to EDP: %s", string(dataToBeSent))
		// TODO Uncomment Sending to EDP
		// retry may make sense
		//resp, err := p.EDPClient.Send(dataToBeSent)
		//if err != nil {
		//	p.Logger.Errorf("failed to send event stream data: %+v for shoot: %s", result.Err, result.Output.ShootName)
		//	continue
		//}
		//if !isSuccess(resp.StatusCode) {
		//	p.Logger.Errorf("failed to send event stream data: %+v for shoot: %s, with HTTP status code: %d", result.Err, result.Output.ShootName, resp.StatusCode)
		//	continue
		//}

		p.Logger.Infof("processing is done for shoot: %s", result.Output.ShootName)
		p.Logger.Infof("event stream is: %+v", result.Output.Metric)
		// If no err then save it in the store
		if result.Err == nil {
			err = p.addDataToCache(result)
			if err != nil {
				p.Logger.Errorf("failed to save data to cache: %v for shoot: %s", err, result.Output.ShootName)
				return
			}
		}
		log.Infof("successfully saved metric: %v for shoot: %s in store", result.Output.Metric, result.Output.ShootName)
	}
}

func (p Process) RunCron() {
	for {
		runtimesPage, err := p.GetKEBResponse()
		if err != nil {
			log.Errorf("failed to retry to connect to KEB: %v", err)
			continue
		}
		p.Logger.Infof("num of runtimes are: %d", runtimesPage.Count)
		filteredRuntimes := filterRuntimes(*runtimesPage)
		p.Logger.Infof("after filtration: num of runtimes are: %d", filteredRuntimes.Count)

		// Spawn workers to do the job(data processing and take it to EDP) and communicate back to the main routine once done
		var wg sync.WaitGroup
		wg.Add(len(filteredRuntimes.Data))
		for _, skrRuntime := range filteredRuntimes.Data {
			go p.scrapeGardenerCluster(&wg, skrRuntime.ShootName)
		}
		wg.Wait()

		p.Logger.Infof("### next round will start in %v.......", p.CronInterval)
		time.Sleep(p.CronInterval)
	}
}
