package cluster_test_tool

//TODO: fix this
//
//import (
//	"context"
//	"strconv"
//	"sync"
//	"sync/atomic"
//	"testing"
//	"time"
//
//	"github.com/asynkron/protoactor-go/actor"
//	"github.com/asynkron/protoactor-go/cluster"
//	"github.com/stretchr/testify/suite"
//)
//
//type PubSubTestSuite struct {
//	suite.Suite
//	fixture *PubSubClusterFixture
//}
//
//func (suite *PubSubTestSuite) SetupTest() {
//	suite.fixture = NewPubSubClusterFixture(suite.T(), 2, false)
//	suite.fixture.Initialize()
//}
//
//func (suite *PubSubTestSuite) TearDownTest() {
//	suite.fixture.ShutDown()
//}
//
//func (suite *PubSubTestSuite) TestCanDeliverSingleMessages() {
//	subscriberIds := suite.fixture.SubscriberIds("single-test", 20)
//	const topic = "single-test-topic"
//	const numMessages = 100
//
//	suite.fixture.SubscribeAllTo(topic, subscriberIds)
//
//	for i := 0; i < numMessages; i++ {
//		data, err := suite.fixture.PublishData(topic, i)
//		suite.Assert().NoError(err, "message "+strconv.Itoa(i)+" should not has error")
//		suite.Assert().NotNil(data, "response "+strconv.Itoa(i)+" should not be nil")
//	}
//
//	suite.fixture.VerifyAllSubscribersGotAllTheData(subscriberIds, numMessages)
//}
//
//func (suite *PubSubTestSuite) TestCanDeliverMessageBatches() {
//	subscriberIds := suite.fixture.SubscriberIds("batch-test", 20)
//	const topic = "batch-test-topic"
//	const numMessages = 100
//
//	suite.fixture.SubscribeAllTo(topic, subscriberIds)
//
//	for i := 0; i < numMessages/10; i++ {
//		data := intRange(i*10, 10)
//		batch, err := suite.fixture.PublishDataBatch(topic, data)
//		suite.Assert().NoError(err, "message "+strconv.Itoa(i)+" should not has error")
//		suite.Assert().NotNil(batch, "response "+strconv.Itoa(i)+" should not be nil")
//	}
//	suite.fixture.VerifyAllSubscribersGotAllTheData(subscriberIds, numMessages)
//}
//
//func (suite *PubSubTestSuite) TestUnsubscribedActorDoesNotReceiveMessages() {
//	const sub1 = "unsubscribe-test-1"
//	const sub2 = "unsubscribe-test-2"
//	const topic = "unsubscribe-test"
//
//	suite.fixture.SubscribeTo(topic, sub1, PubSubSubscriberKind)
//	suite.fixture.SubscribeTo(topic, sub2, PubSubSubscriberKind)
//
//	suite.fixture.UnSubscribeTo(topic, sub2, PubSubSubscriberKind)
//
//	_, err := suite.fixture.PublishData(topic, 1)
//	suite.Assert().NoError(err, "PublishData should not has error")
//
//	time.Sleep(time.Second * 1) // give time for the message "not to be delivered" to second subscriber
//	WaitUntil(suite.T(), func() bool {
//		suite.fixture.DeliveriesLock.RLock()
//		defer suite.fixture.DeliveriesLock.RUnlock()
//		return len(suite.fixture.Deliveries) == 1
//	}, "only one delivery should happen because the other actor is unsubscribed", DefaultWaitTimeout)
//
//	suite.fixture.DeliveriesLock.RLock()
//	defer suite.fixture.DeliveriesLock.RUnlock()
//	suite.Assert().Len(suite.fixture.Deliveries, 1, "only one delivery should happen because the other actor is unsubscribed")
//	suite.Assert().Equal(sub1, suite.fixture.Deliveries[0].Identity, "the other actor should be unsubscribed")
//}
//
//func (suite *PubSubTestSuite) TestCanSubscribeWithPid() {
//	const topic = "pid-subscribe"
//
//	var deliveredMessage *DataPublished
//
//	props := actor.PropsFromFunc(func(context actor.Context) {
//		switch msg := context.Message().(type) {
//		case *DataPublished:
//			deliveredMessage = msg
//		}
//	})
//	member := suite.fixture.GetMembers()[0]
//	pid := member.ActorSystem.Root.Spawn(props)
//	_, err := member.SubscribeByPid(topic, pid)
//	suite.Assert().NoError(err, "SubscribeByPid should not has error")
//
//	_, err = suite.fixture.PublishData(topic, 1)
//	suite.Assert().NoError(err, "PublishData should not has error")
//
//	WaitUntil(suite.T(), func() bool {
//		return deliveredMessage != nil
//	}, "message should be delivered", DefaultWaitTimeout)
//	suite.Assert().EqualValues(1, deliveredMessage.Data)
//}
//
//func (suite *PubSubTestSuite) TestCanUnsubscribeWithPid() {
//	const topic = "pid-unsubscribe"
//
//	var deliveryCount int32 = 0
//
//	props := actor.PropsFromFunc(func(context actor.Context) {
//		switch context.Message().(type) {
//		case *DataPublished:
//			atomic.AddInt32(&deliveryCount, 1)
//		}
//	})
//	member := suite.fixture.GetMembers()[0]
//	pid := member.ActorSystem.Root.Spawn(props)
//	_, err := member.SubscribeByPid(topic, pid)
//	suite.Assert().NoError(err, "SubscribeByPid should not has error")
//
//	_, err = member.UnsubscribeByPid(topic, pid)
//	suite.Assert().NoError(err, "UnsubscribeByPid should not has error")
//
//	_, err = suite.fixture.PublishData(topic, 1)
//	suite.Assert().NoError(err, "PublishData should not has error")
//
//	time.Sleep(time.Second * 1) // give time for the message "not to be delivered" to second subscriber
//	suite.Assert().EqualValues(0, deliveryCount, "message should not be delivered")
//}
//
//func (suite *PubSubTestSuite) TestStoppedActorThatDidNotUnsubscribeDoesNotBlockPublishingToTopic() {
//	const topic = "missing-unsubscribe"
//	var deliveryCount int32 = 0
//
//	// this scenario is only relevant for regular actors,
//	// virtual actors always exist, so the msgs should never be deadlettered
//	props := actor.PropsFromFunc(func(context actor.Context) {
//		switch context.Message().(type) {
//		case *DataPublished:
//			atomic.AddInt32(&deliveryCount, 1)
//		}
//	})
//	member := suite.fixture.GetMembers()[0]
//	pid1 := member.ActorSystem.Root.Spawn(props)
//	pid2 := member.ActorSystem.Root.Spawn(props)
//
//	// spawn two actors and subscribe them to the topic
//	_, err := member.SubscribeByPid(topic, pid1)
//	suite.Assert().NoError(err, "SubscribeByPid1 should not has error")
//	_, err = member.SubscribeByPid(topic, pid2)
//	suite.Assert().NoError(err, "SubscribeByPid2 should not has error")
//
//	// publish one message
//	_, err = suite.fixture.PublishData(topic, 1)
//	suite.Assert().NoError(err, "PublishData should not has error")
//
//	WaitUntil(suite.T(), func() bool {
//		return atomic.LoadInt32(&deliveryCount) == 2
//	}, "both messages should be delivered", DefaultWaitTimeout)
//
//	// kill one of the actors
//	member.ActorSystem.Root.Stop(pid2)
//
//	// publish again
//	_, err = suite.fixture.PublishData(topic, 2)
//	suite.Assert().NoError(err, "PublishData should not has error")
//
//	WaitUntil(suite.T(), func() bool {
//		return atomic.LoadInt32(&deliveryCount) == 3
//	}, "second publish should be delivered only to one of the actors", DefaultWaitTimeout)
//
//	WaitUntil(suite.T(), func() bool {
//		subscribers, err := suite.fixture.GetSubscribersForTopic(topic)
//		suite.Assert().NoError(err, "GetSubscribersForTopic should not has error")
//
//		hasPid2 := false
//		for _, subscriber := range subscribers.Subscribers {
//			if subscriber.GetPid() != nil &&
//				subscriber.GetPid().Id == pid2.Id &&
//				subscriber.GetPid().Address == pid2.Address {
//				hasPid2 = true
//				break
//			}
//		}
//		return !hasPid2
//	}, "pid2 should be removed from subscriber store", DefaultWaitTimeout*1000)
//}
//
//func (suite *PubSubTestSuite) TestSlowPidSubscriberThatTimesOutDoesNotPreventSubsequentPublishes() {
//	const topic = "slow-pid-subscriber"
//	var deliveryCount int32 = 0
//
//	// a slow subscriber that will timeout
//	props := actor.PropsFromFunc(func(context actor.Context) {
//		time.Sleep(time.Second * 4)
//		atomic.AddInt32(&deliveryCount, 1)
//	})
//
//	member := suite.fixture.RandomMember()
//	pid := member.ActorSystem.Root.Spawn(props)
//	_, err := member.SubscribeByPid(topic, pid)
//	suite.Assert().NoError(err, "SubscribeByPid should not has error")
//
//	// publish one message
//	_, err = suite.fixture.PublishData(topic, 1)
//	suite.Assert().NoError(err, "PublishData should not has error")
//
//	// next published message should also be delivered
//	_, err = suite.fixture.PublishData(topic, 1)
//	suite.Assert().NoError(err, "PublishData should not has error")
//
//	WaitUntil(suite.T(), func() bool {
//		return atomic.LoadInt32(&deliveryCount) == 2
//	}, "A timing out subscriber should not prevent subsequent publishes", time.Second*10)
//}
//
//func (suite *PubSubTestSuite) TestSlowClusterIdentitySubscriberThatTimesOutDoesNotPreventSubsequentPublishes() {
//	const topic = "slow-ci-subscriber"
//	suite.fixture.SubscribeTo(topic, "slow-ci-1", PubSubTimeoutSubscriberKind)
//
//	// publish one message
//	_, err := suite.fixture.PublishData(topic, 1)
//	suite.Assert().NoError(err, "PublishData1 should not has error")
//
//	// next published message should also be delivered
//	_, err = suite.fixture.PublishData(topic, 1)
//	suite.Assert().NoError(err, "PublishData2 should not has error")
//
//	WaitUntil(suite.T(), func() bool {
//		suite.fixture.DeliveriesLock.RLock()
//		defer suite.fixture.DeliveriesLock.RUnlock()
//
//		return len(suite.fixture.Deliveries) == 2
//	}, "A timing out subscriber should not prevent subsequent publishes", time.Second*10)
//}
//
//func (suite *PubSubTestSuite) TestCanPublishMessagesViaBatchingProducer() {
//	subscriberIds := suite.fixture.SubscriberIds("batching-producer-test", 20)
//	const topic = "batching-producer"
//	const numMessages = 100
//
//	suite.fixture.SubscribeAllTo(topic, subscriberIds)
//
//	producer := suite.fixture.GetMembers()[0].BatchingProducer(topic, cluster.WithBatchingProducerBatchSize(10))
//	defer producer.Dispose()
//
//	wg := sync.WaitGroup{}
//	for i := 0; i < numMessages; i++ {
//		wg.Add(1)
//		go func(i int) {
//			defer wg.Done()
//			_, err := producer.Produce(context.Background(), &DataPublished{Data: int32(i)})
//			suite.Assert().NoError(err, "Produce should not has error")
//		}(i)
//	}
//	wg.Wait()
//
//	suite.fixture.VerifyAllSubscribersGotAllTheData(subscriberIds, numMessages)
//}
//
//func (suite *PubSubTestSuite) TestCanPublishMessagesViaBatchingProducerWithCustomQueue() {
//	subscriberIds := suite.fixture.SubscriberIds("batching-producer-test-with-chan", 20)
//	const topic = "batching-producer-with-chan"
//	const numMessages = 100
//
//	suite.fixture.SubscribeAllTo(topic, subscriberIds)
//
//	producer := suite.fixture.GetMembers()[0].BatchingProducer(topic, cluster.WithBatchingProducerBatchSize(10), cluster.WithBatchingProducerMaxQueueSize(2000))
//	defer producer.Dispose()
//
//	wg := sync.WaitGroup{}
//	for i := 0; i < numMessages; i++ {
//		wg.Add(1)
//		go func(i int) {
//			defer wg.Done()
//			_, err := producer.Produce(context.Background(), &DataPublished{Data: int32(i)})
//			suite.Assert().NoError(err, "Produce should not has error")
//		}(i)
//	}
//	wg.Wait()
//
//	suite.fixture.VerifyAllSubscribersGotAllTheData(subscriberIds, numMessages)
//}
//
//func (suite *PubSubTestSuite) TestWillExpireTopicActorAfterIdle() {
//	subscriberIds := suite.fixture.SubscriberIds("batching-producer-idl-test", 20)
//	const topic = "batching-producer"
//	const numMessages = 100
//
//	suite.fixture.SubscribeAllTo(topic, subscriberIds)
//
//	firstCluster := suite.fixture.GetMembers()[0]
//
//	producer := firstCluster.BatchingProducer(topic, cluster.WithBatchingProducerPublisherIdleTimeout(time.Second*2))
//	defer producer.Dispose()
//
//	wg := sync.WaitGroup{}
//	for i := 0; i < numMessages; i++ {
//		wg.Add(1)
//		go func(i int) {
//			defer wg.Done()
//			_, err := producer.Produce(context.Background(), &DataPublished{Data: int32(i)})
//			suite.Assert().NoError(err, "Produce should not has error")
//		}(i)
//	}
//	wg.Wait()
//
//	pid := firstCluster.Get(topic, cluster.TopicActorKind)
//	suite.Assert().NotNil(pid, "Topic actor should not be nil")
//
//	time.Sleep(time.Second * 5)
//
//	newPid := firstCluster.Get(topic, cluster.TopicActorKind)
//	suite.Assert().NotEqual(pid.String(), newPid.String(), "Topic actor should be recreated")
//}
//
//// In order for 'go test' to run this suite, we need to create
//// a normal test function and pass our suite to suite.Run
//func TestPubSubTestSuite(t *testing.T) {
//	suite.Run(t, new(PubSubTestSuite))
//}
//
//func intRange(start int, count int) []int {
//	res := make([]int, count)
//	for i := 0; i < count; i++ {
//		res[i] = start + i
//	}
//	return res
//}
