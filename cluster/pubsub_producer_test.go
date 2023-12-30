package cluster

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/proto"
)

type PubSubBatchingProducerTestSuite struct {
	suite.Suite
	batchesSent []*PubSubBatch
}

func (suite *PubSubBatchingProducerTestSuite) SetupTest() {
	suite.batchesSent = make([]*PubSubBatch, 0)
}

func (suite *PubSubBatchingProducerTestSuite) allSentNumbersShouldEqual(batchesSent []*PubSubBatch, nums ...int) {
	allNumbers := make([]int, 0, len(nums))
	for _, batch := range batchesSent {
		for _, envelope := range batch.Envelopes {
			allNumbers = append(allNumbers, int(envelope.(*TestMessage).Number))
		}
	}
	suite.Assert().ElementsMatch(nums, allNumbers)
}

func (suite *PubSubBatchingProducerTestSuite) iter(from, to int) []int {
	nums := make([]int, 0, to-from)
	for i := from; i < to; i++ {
		nums = append(nums, i)
	}
	return nums
}

func (suite *PubSubBatchingProducerTestSuite) record(batch *PubSubBatch) (*PublishResponse, error) {
	b := &PubSubBatch{Envelopes: make([]proto.Message, 0, len(batch.Envelopes))}
	b.Envelopes = append(b.Envelopes, batch.Envelopes...)

	suite.batchesSent = append(suite.batchesSent, b)
	return &PublishResponse{Status: PublishStatus_Ok}, nil
}

func (suite *PubSubBatchingProducerTestSuite) wait(_ *PubSubBatch) (*PublishResponse, error) {
	time.Sleep(time.Second * 1)
	return &PublishResponse{Status: PublishStatus_Ok}, nil
}

func (suite *PubSubBatchingProducerTestSuite) waitThenFail(_ *PubSubBatch) (*PublishResponse, error) {
	time.Sleep(time.Millisecond * 500)
	return &PublishResponse{Status: PublishStatus_Failed}, &testException{}
}

func (suite *PubSubBatchingProducerTestSuite) fail(_ *PubSubBatch) (*PublishResponse, error) {
	return &PublishResponse{Status: PublishStatus_Failed}, &testException{}
}

func (suite *PubSubBatchingProducerTestSuite) failTimesThenSucceed(times int) func(*PubSubBatch) (*PublishResponse, error) {
	count := 0
	return func(batch *PubSubBatch) (*PublishResponse, error) {
		count++
		if count <= times {
			return &PublishResponse{Status: PublishStatus_Failed}, &testException{}
		}
		return suite.record(batch)
	}
}

func (suite *PubSubBatchingProducerTestSuite) timeout() (*PublishResponse, error) {
	return nil, nil
}

func (suite *PubSubBatchingProducerTestSuite) TestProducerSendsMessagesInBatches() {
	producer := NewBatchingProducer(newMockPublisher(suite.record), "topic", WithBatchingProducerBatchSize(10))
	defer producer.Dispose()

	infos := make([]*ProduceProcessInfo, 0, 10000)
	for i := 0; i < 10000; i++ {
		info, err := producer.Produce(context.Background(), &TestMessage{Number: int32(i)})
		suite.Assert().NoError(err)
		infos = append(infos, info)
	}
	for _, info := range infos {
		<-info.Finished
		suite.Assert().Nil(info.Err)
	}

	anyBatchesEnvelopesCountIsGreaterThanOne := false
	for _, batch := range suite.batchesSent {
		if len(batch.Envelopes) > 1 {
			anyBatchesEnvelopesCountIsGreaterThanOne = true
			break
		}
	}
	suite.Assert().True(anyBatchesEnvelopesCountIsGreaterThanOne, "messages should be batched")

	allBatchesEnvelopeCountAreLessThanBatchSize := true
	for _, batch := range suite.batchesSent {
		if len(batch.Envelopes) > 10 {
			allBatchesEnvelopeCountAreLessThanBatchSize = false
			break
		}
	}
	suite.Assert().True(allBatchesEnvelopeCountAreLessThanBatchSize, "batches should not exceed configured size")

	suite.allSentNumbersShouldEqual(suite.batchesSent, suite.iter(0, 10000)...)
}

func (suite *PubSubBatchingProducerTestSuite) TestPublishingThroughStoppedProducerThrows() {
	producer := NewBatchingProducer(newMockPublisher(suite.record), "topic", WithBatchingProducerBatchSize(10))
	producer.Dispose()

	_, err := producer.Produce(context.Background(), &TestMessage{Number: 1})
	suite.Assert().ErrorIs(err, &InvalidOperationException{Topic: "topic"})
}

func (suite *PubSubBatchingProducerTestSuite) TestAllPendingTasksCompleteWhenProducerIsStopped() {
	provider := NewBatchingProducer(newMockPublisher(suite.wait), "topic", WithBatchingProducerBatchSize(5))

	infoList := make([]*ProduceProcessInfo, 0, 100)
	for i := 0; i < 100; i++ {
		info, err := provider.Produce(context.Background(), &TestMessage{Number: int32(i)})
		suite.Assert().NoError(err)
		infoList = append(infoList, info)
	}

	provider.Dispose()

	for _, info := range infoList {
		<-info.Finished
		suite.Assert().Nil(info.Err)
	}
}

func (suite *PubSubBatchingProducerTestSuite) TestAllPendingTasksCompleteWhenProducerFails() {
	producer := NewBatchingProducer(newMockPublisher(suite.waitThenFail), "topic", WithBatchingProducerBatchSize(5))
	defer producer.Dispose()

	infoList := make([]*ProduceProcessInfo, 0, 100)
	for i := 0; i < 100; i++ {
		info, err := producer.Produce(context.Background(), &TestMessage{Number: int32(i)})
		suite.Assert().NoError(err)
		infoList = append(infoList, info)
	}

	for _, info := range infoList {
		<-info.Finished
		suite.Assert().Error(info.Err)
	}
}

func (suite *PubSubBatchingProducerTestSuite) TestPublishingThroughFailedProducerThrows() {
	producer := NewBatchingProducer(newMockPublisher(suite.fail), "topic", WithBatchingProducerBatchSize(10))
	defer producer.Dispose()

	info, err := producer.Produce(context.Background(), &TestMessage{Number: 1})
	suite.Assert().NoError(err)
	<-info.Finished
	suite.Assert().ErrorIs(info.Err, &testException{})

	_, err = producer.Produce(context.Background(), &TestMessage{Number: 1})
	suite.Assert().ErrorIs(err, &InvalidOperationException{Topic: "topic"})
}

func (suite *PubSubBatchingProducerTestSuite) TestThrowsWhenQueueFull() {
	producer := NewBatchingProducer(newMockPublisher(suite.record), "topic", WithBatchingProducerBatchSize(1), WithBatchingProducerMaxQueueSize(10))
	defer producer.Dispose()

	hasError := false
	for i := 0; i < 20; i++ {
		_, err := producer.Produce(context.Background(), &TestMessage{Number: int32(i)})
		if err != nil {
			hasError = true
			suite.Assert().ErrorIs(err, &ProducerQueueFullException{})
		}
	}
	suite.Assert().True(hasError)
}

func (suite *PubSubBatchingProducerTestSuite) TestCanCancelPublishingAMessage() {
	producer := NewBatchingProducer(newMockPublisher(suite.record), "topic", WithBatchingProducerBatchSize(1), WithBatchingProducerMaxQueueSize(10))
	defer producer.Dispose()

	messageWithoutCancellation := &TestMessage{Number: 1}
	t1, err := producer.Produce(context.Background(), messageWithoutCancellation)
	suite.Assert().NoError(err)

	ctx, cancel := context.WithCancel(context.Background())
	t2, err := producer.Produce(ctx, &TestMessage{Number: 2})
	cancel()
	suite.Assert().NoError(err)

	<-t1.Finished
	suite.Assert().NoError(t1.Err)
	<-t2.Finished
	suite.Assert().True(t2.IsCancelled())

	suite.allSentNumbersShouldEqual(suite.batchesSent, 1)
}

func (suite *PubSubBatchingProducerTestSuite) TestCanRetryOnPublishingError() {
	retries := make([]int, 0, 10)
	producer := NewBatchingProducer(newMockPublisher(suite.failTimesThenSucceed(3)), "topic",
		WithBatchingProducerBatchSize(1),
		WithBatchingProducerOnPublishingError(func(retry int, e error, batch *PubSubBatch) *PublishingErrorDecision {
			retries = append(retries, retry)
			return RetryBatchImmediately
		}))
	defer producer.Dispose()

	info, err := producer.Produce(context.Background(), &TestMessage{Number: 1})
	suite.Assert().NoError(err)

	<-info.Finished
	suite.Assert().Equal([]int{1, 2, 3}, retries)
}

func (suite *PubSubBatchingProducerTestSuite) TestCanSkipBatchOnPublishingError() {
	producer := NewBatchingProducer(newMockPublisher(suite.failTimesThenSucceed(1)), "topic",
		WithBatchingProducerBatchSize(1),
		WithBatchingProducerOnPublishingError(func(retry int, e error, batch *PubSubBatch) *PublishingErrorDecision {
			return FailBatchAndContinue
		}))
	defer producer.Dispose()

	t1, err := producer.Produce(context.Background(), &TestMessage{Number: 1})
	suite.Assert().NoError(err)

	t2, err := producer.Produce(context.Background(), &TestMessage{Number: 2})
	suite.Assert().NoError(err)

	<-t1.Finished
	suite.Assert().ErrorIs(t1.Err, &testException{})
	<-t2.Finished
	suite.Assert().NoError(t2.Err)
}

func (suite *PubSubBatchingProducerTestSuite) TestCanStopProducerWhenRetryingInfinitely() {
	producer := NewBatchingProducer(newMockPublisher(suite.fail), "topic",
		WithBatchingProducerBatchSize(1),
		WithBatchingProducerOnPublishingError(func(retry int, e error, batch *PubSubBatch) *PublishingErrorDecision {
			return RetryBatchImmediately
		}))

	t1, err := producer.Produce(context.Background(), &TestMessage{Number: 1})
	suite.Assert().NoError(err)

	time.Sleep(50 * time.Millisecond)
	producer.Dispose()
	suite.Assert().True(t1.IsCancelled())
}

func (suite *PubSubBatchingProducerTestSuite) TestIfMessageIsCancelledMeanwhileRetryingItIsNotPublished() {
	publisher := newOptionalFailureMockPublisher(true)
	producer := NewBatchingProducer(publisher, "topic",
		WithBatchingProducerBatchSize(1),
		WithBatchingProducerOnPublishingError(func(retry int, e error, batch *PubSubBatch) *PublishingErrorDecision {
			return RetryBatchImmediately
		}))
	defer producer.Dispose()
	ctx, cancel := context.WithCancel(context.Background())
	t1, err := producer.Produce(ctx, &TestMessage{Number: 1})
	suite.Assert().NoError(err)

	// give it a moment to spin
	time.Sleep(50 * time.Millisecond)

	// cancel the message publish
	cancel()
	<-t1.Finished
	suite.Assert().True(t1.IsCancelled())

	suite.Assert().Len(publisher.sentBatches, 0)
	publisher.shouldFail = false
	t2, err := producer.Produce(context.Background(), &TestMessage{Number: 2})
	suite.Assert().NoError(err)
	<-t2.Finished
	suite.Assert().NoError(t2.Err)

	suite.allSentNumbersShouldEqual(publisher.sentBatches, 2)
}

func (suite *PubSubBatchingProducerTestSuite) TestCanHandlePublishTimeouts() {
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestPubSubBatchingTestSuite(t *testing.T) {
	suite.Run(t, new(PubSubBatchingProducerTestSuite))
}

type mockPublisher struct {
	publish func(*PubSubBatch) (*PublishResponse, error)
}

func (m *mockPublisher) Logger() *slog.Logger {
	return slog.Default()
}

func newMockPublisher(publish func(*PubSubBatch) (*PublishResponse, error)) *mockPublisher {
	return &mockPublisher{publish: publish}
}

func (m *mockPublisher) Initialize(_ context.Context, topic string, config PublisherConfig) (*Acknowledge, error) {
	return &Acknowledge{}, nil
}

func (m *mockPublisher) PublishBatch(_ context.Context, topic string, batch *PubSubBatch, opts ...GrainCallOption) (*PublishResponse, error) {
	return m.publish(batch)
}

func (m *mockPublisher) Publish(_ context.Context, topic string, message proto.Message, opts ...GrainCallOption) (*PublishResponse, error) {
	return m.publish(&PubSubBatch{Envelopes: []proto.Message{message}})
}

type optionalFailureMockPublisher struct {
	sentBatches []*PubSubBatch
	shouldFail  bool
}

func (o *optionalFailureMockPublisher) Logger() *slog.Logger {
	return slog.Default()
}

// newOptionalFailureMockPublisher creates a mock publisher that can be configured to fail or not
func newOptionalFailureMockPublisher(shouldFail bool) *optionalFailureMockPublisher {
	return &optionalFailureMockPublisher{shouldFail: shouldFail}
}

func (o *optionalFailureMockPublisher) Initialize(ctx context.Context, topic string, config PublisherConfig) (*Acknowledge, error) {
	return &Acknowledge{}, nil
}

func (o *optionalFailureMockPublisher) PublishBatch(ctx context.Context, topic string, batch *PubSubBatch, opts ...GrainCallOption) (*PublishResponse, error) {
	if o.shouldFail {
		return nil, &testException{}
	}
	copiedBatch := &PubSubBatch{Envelopes: make([]proto.Message, len(batch.Envelopes))}
	copy(copiedBatch.Envelopes, batch.Envelopes)

	o.sentBatches = append(o.sentBatches, copiedBatch)
	return &PublishResponse{}, nil
}

func (o *optionalFailureMockPublisher) Publish(ctx context.Context, topic string, message proto.Message, opts ...GrainCallOption) (*PublishResponse, error) {
	return o.PublishBatch(ctx, topic, &PubSubBatch{Envelopes: []proto.Message{message}}, opts...)
}

type testException struct{}

func (t *testException) Error() string {
	return "test exception"
}

func (t *testException) Is(err error) bool {
	_, ok := err.(*testException)
	return ok
}
