package process

import (
	"fmt"
	"testing"

	"github.com/kyma-incubator/metris/pkg/edp"

	"github.com/google/uuid"

	metriscache "github.com/kyma-incubator/metris/pkg/cache"
	metristesting "github.com/kyma-incubator/metris/pkg/testing"

	"github.com/onsi/gomega"

	gocache "github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/util/workqueue"

	kebruntime "github.com/kyma-project/control-plane/components/kyma-environment-broker/common/runtime"
)

func TestGetOldRecordIfMetricExists(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	cache := gocache.New(gocache.NoExpiration, gocache.NoExpiration)
	expectedSubAccIDToExist := uuid.New().String()
	expectedRecord := metriscache.Record{
		SubAccountID: expectedSubAccIDToExist,
		ShootName:    fmt.Sprintf("shoot-%s", metristesting.GenerateRandomAlphaString(5)),
		KubeConfig:   "foo",
		Metric:       NewMetric(),
	}
	expectedSubAccIDWithNoMetrics := uuid.New().String()
	recordsToBeAdded := []metriscache.Record{
		expectedRecord,
		{
			SubAccountID: uuid.New().String(),
			ShootName:    fmt.Sprintf("shoot-%s", metristesting.GenerateRandomAlphaString(5)),
			KubeConfig:   "foo",
		},
		{
			SubAccountID: expectedSubAccIDWithNoMetrics,
			ShootName:    "",
			KubeConfig:   "",
		},
	}
	for _, record := range recordsToBeAdded {
		err := cache.Add(record.SubAccountID, record, gocache.NoExpiration)
		g.Expect(err).Should(gomega.BeNil())
	}

	p := Process{
		Cache:  cache,
		Logger: logrus.New(),
	}

	t.Run("old metric found for a subAccountID", func(t *testing.T) {
		gotRecord, err := p.getOldRecordIfMetricExists(expectedSubAccIDToExist)
		g.Expect(err).Should(gomega.BeNil())
		g.Expect(*gotRecord).To(gomega.Equal(expectedRecord))
	})

	t.Run("old metric not found for a subAccountID", func(t *testing.T) {
		subAccIDWhichDoesNotExist := uuid.New().String()
		_, err := p.getOldRecordIfMetricExists(subAccIDWhichDoesNotExist)
		g.Expect(err).ShouldNot(gomega.BeNil())
		g.Expect(err.Error()).To(gomega.Equal(fmt.Sprintf("subAccountID: %s not found", subAccIDWhichDoesNotExist)))
	})

	t.Run("old metric found for a subAccountID but does not have metric", func(t *testing.T) {
		_, err := p.getOldRecordIfMetricExists(expectedSubAccIDWithNoMetrics)
		g.Expect(err).ShouldNot(gomega.BeNil())
		g.Expect(err.Error()).To(gomega.Equal(fmt.Sprintf("old metrics for subAccountID: %s not found", expectedSubAccIDWithNoMetrics)))
	})
}

func TestPollKEBForRuntimes(t *testing.T) {

}

func TestPopulateCacheAndQueue(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	t.Run("runtimes with only provisioned status and other statuses with failures", func(t *testing.T) {
		provisionedSuccessfullySubAccIDs := []string{uuid.New().String(), uuid.New().String()}
		provisionedFailedSubAccIDs := []string{uuid.New().String(), uuid.New().String()}
		cache := gocache.New(gocache.NoExpiration, gocache.NoExpiration)
		queue := workqueue.NewDelayingQueue()
		p := Process{
			Queue:  queue,
			Cache:  cache,
			Logger: logrus.New(),
		}
		runtimesPage := new(kebruntime.RuntimesPage)
		shootID := 0

		expectedQueue := workqueue.NewDelayingQueue()
		expectedCache := gocache.New(gocache.NoExpiration, gocache.NoExpiration)

		runtimesPage, expectedCache, expectedQueue, err := AddSuccessfulIDsToCacheQueueAndRuntimes(runtimesPage, provisionedSuccessfullySubAccIDs, expectedCache, expectedQueue)
		g.Expect(err).Should(gomega.BeNil())

		for _, failedID := range provisionedFailedSubAccIDs {
			runtime := metristesting.NewRuntimesDTO(failedID, fmt.Sprintf("shoot-%d", shootID), metristesting.WithFailedState)
			runtimesPage.Data = append(runtimesPage.Data, runtime)
			shootID += 1
		}

		p.populateCacheAndQueue(runtimesPage)
		g.Expect(*p.Cache).To(gomega.Equal(*expectedCache))
		g.Expect(p.Queue.Len()).To(gomega.Equal(expectedQueue.Len()))
		for expectedQueue.Len() > 0 {
			gotItem, _ := p.Queue.Get()
			expectedItem, _ := expectedQueue.Get()
			g.Expect(gotItem).To(gomega.Equal(expectedItem))
		}
	})

	t.Run("runtimes with both provisioned and deprovisioned status", func(t *testing.T) {
		provisionedSuccessfullySubAccIDs := []string{uuid.New().String(), uuid.New().String()}
		provisionedAndDeprovisionedSubAccIDs := []string{uuid.New().String(), uuid.New().String()}
		cache := gocache.New(gocache.NoExpiration, gocache.NoExpiration)
		queue := workqueue.NewDelayingQueue()
		p := Process{
			Queue:  queue,
			Cache:  cache,
			Logger: logrus.New(),
		}
		runtimesPage := new(kebruntime.RuntimesPage)

		expectedQueue := workqueue.NewDelayingQueue()
		expectedCache := gocache.New(gocache.NoExpiration, gocache.NoExpiration)

		runtimesPage, expectedCache, expectedQueue, err := AddSuccessfulIDsToCacheQueueAndRuntimes(runtimesPage, provisionedSuccessfullySubAccIDs, expectedCache, expectedQueue)
		g.Expect(err).Should(gomega.BeNil())

		for _, failedID := range provisionedAndDeprovisionedSubAccIDs {
			runtime := metristesting.NewRuntimesDTO(failedID, fmt.Sprintf("shoot-%s", metristesting.GenerateRandomAlphaString(5)), metristesting.WithProvisionedAndDeprovisionedState)
			runtimesPage.Data = append(runtimesPage.Data, runtime)
		}

		p.populateCacheAndQueue(runtimesPage)
		g.Expect(*p.Cache).To(gomega.Equal(*expectedCache))
		g.Expect(AreQueuesEqual(p.Queue, expectedQueue)).To(gomega.BeTrue())
	})

	t.Run("with loaded cache but shoot name changed", func(t *testing.T) {
		subAccID := uuid.New().String()
		cache := gocache.New(gocache.NoExpiration, gocache.NoExpiration)
		queue := workqueue.NewDelayingQueue()
		oldShootName := fmt.Sprintf("shoot-%s", metristesting.GenerateRandomAlphaString(5))
		newShootName := fmt.Sprintf("shoot-%s", metristesting.GenerateRandomAlphaString(5))

		p := Process{
			Queue:  queue,
			Cache:  cache,
			Logger: logrus.New(),
		}
		oldRecord := NewRecord(subAccID, oldShootName, "foo")
		newRecord := NewRecord(subAccID, newShootName, "")

		err := p.Cache.Add(subAccID, oldRecord, gocache.NoExpiration)
		g.Expect(err).Should(gomega.BeNil())

		runtimesPage := new(kebruntime.RuntimesPage)
		expectedQueue := workqueue.NewDelayingQueue()
		expectedCache := gocache.New(gocache.NoExpiration, gocache.NoExpiration)
		err = expectedCache.Add(subAccID, newRecord, gocache.NoExpiration)
		g.Expect(err).Should(gomega.BeNil())

		runtime := metristesting.NewRuntimesDTO(subAccID, newShootName, metristesting.WithSucceededState)
		runtimesPage.Data = append(runtimesPage.Data, runtime)

		p.populateCacheAndQueue(runtimesPage)
		g.Expect(*p.Cache).To(gomega.Equal(*expectedCache))
		g.Expect(AreQueuesEqual(p.Queue, expectedQueue)).To(gomega.BeTrue())
	})

}

func NewRecord(subAccId, shootName, kubeconfig string) metriscache.Record {
	return metriscache.Record{
		SubAccountID: subAccId,
		ShootName:    shootName,
		KubeConfig:   kubeconfig,
		Metric:       nil,
	}
}

func TestExecute(t *testing.T) {

}

func AreQueuesEqual(src, dest workqueue.DelayingInterface) bool {
	if src.Len() != dest.Len() {
		return false
	}
	for src.Len() > 0 {
		srcItem, _ := src.Get()
		destItem, _ := dest.Get()
		if srcItem != destItem {
			return false
		}
	}
	return true
}

func NewMetric() *edp.ConsumptionMetrics {
	return &edp.ConsumptionMetrics{
		Compute: edp.Compute{
			VMTypes: []edp.VMType{
				{
					Name:  "standard_a2_v2",
					Count: 2,
				},
			},
			ProvisionedCpus:  3,
			ProvisionedRAMGb: 40,
		},
	}
}

func AddSuccessfulIDsToCacheQueueAndRuntimes(runtimesPage *kebruntime.RuntimesPage, successfulIDs []string, expectedCache *gocache.Cache, expectedQueue workqueue.DelayingInterface) (*kebruntime.RuntimesPage, *gocache.Cache, workqueue.DelayingInterface, error) {
	for _, successfulID := range successfulIDs {
		shootID := metristesting.GenerateRandomAlphaString(5)
		shootName := fmt.Sprintf("shoot-%s", shootID)
		runtime := metristesting.NewRuntimesDTO(successfulID, shootName, metristesting.WithSucceededState)
		runtimesPage.Data = append(runtimesPage.Data, runtime)
		err := expectedCache.Add(successfulID, metriscache.Record{
			SubAccountID: successfulID,
			ShootName:    shootName,
		}, gocache.NoExpiration)
		if err != nil {
			return nil, nil, nil, err
		}
		expectedQueue.Add(successfulID)
	}
	return runtimesPage, expectedCache, expectedQueue, nil
}
