package cluster

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/internal/queue/mpsc"
	"github.com/asynkron/protoactor-go/log"
	"golang.org/x/net/context"
)

// PublishingErrorHandler decides what to do with a publishing error in BatchingProducer
type PublishingErrorHandler func(retries int, e error, batch *PubSubBatch) *PublishingErrorDecision

type BatchingProducerConfig struct {
	// Maximum size of the published batch. Default: 2000.
	BatchSize int
	// Max size of the requests waiting in queue. If value is provided, the producer will throw
	// ProducerQueueFullException when queue size is exceeded. If 0 or unset, the queue is unbounded
	// Note that bounded queue has better performance than unbounded queue.
	// Default: 0 (unbounded)
	MaxQueueSize int

	// How long to wait for the publishing to complete.
	// Default: 5s
	PublishTimeout time.Duration

	// Error handler that can decide what to do with an error when publishing a batch.
	// Default: Fail and stop the BatchingProducer
	OnPublishingError PublishingErrorHandler

	// A throttle for logging from this producer. By default, a throttle shared between all instances of
	// BatchingProducer is used, that allows for 10 events in 1 second.
	LogThrottle actor.ShouldThrottle

	// Optional idle timeout which will specify to the `IPublisher` how long it should wait before invoking clean
	// up code to recover resources.
	PublisherIdleTimeout time.Duration
}

var defaultBatchingProducerLogThrottle = actor.NewThrottle(10, time.Second, func(i int32) {
	plog.Info("[BatchingProducer] Throttled logs", log.Int("count", int(i)))
})

func newBatchingProducerConfig(opts ...BatchingProducerConfigOption) *BatchingProducerConfig {
	config := &BatchingProducerConfig{
		BatchSize:      2000,
		PublishTimeout: 5 * time.Second,
		OnPublishingError: func(retries int, e error, batch *PubSubBatch) *PublishingErrorDecision {
			return FailBatchAndStop
		},
		LogThrottle: defaultBatchingProducerLogThrottle,
	}

	for _, opt := range opts {
		opt(config)
	}

	return config
}

type BatchingProducer struct {
	config           *BatchingProducerConfig
	topic            string
	publisher        Publisher
	publisherChannel channel[produceMessage]
	loopCancel       context.CancelFunc
	loopDone         chan struct{}
	msgLeft          uint32
}

func NewBatchingProducer(publisher Publisher, topic string, opts ...BatchingProducerConfigOption) *BatchingProducer {
	config := newBatchingProducerConfig(opts...)
	p := &BatchingProducer{
		config:    config,
		topic:     topic,
		publisher: publisher,
		msgLeft:   0,
		loopDone:  make(chan struct{}),
	}
	if config.MaxQueueSize > 0 {
		p.publisherChannel = newBoundedChannel[produceMessage](config.MaxQueueSize)
	} else {
		p.publisherChannel = newUnboundedChannel[produceMessage]()
	}
	ctx, cancelFunc := context.WithCancel(context.Background())
	p.loopCancel = cancelFunc
	go p.publishLoop(ctx)

	return p
}

type pubsubBatchWithReceipts struct {
	batch  *PubSubBatch
	ctxArr []context.Context
}

// newPubSubBatchWithReceipts creates a new pubsubBatchWithReceipts
func newPubSubBatchWithReceipts() *pubsubBatchWithReceipts {
	return &pubsubBatchWithReceipts{
		batch:  &PubSubBatch{Envelopes: make([]interface{}, 0, 10)},
		ctxArr: make([]context.Context, 0, 10),
	}
}

type produceMessage struct {
	message interface{}
	ctx     context.Context
}

// Dispose stops the producer and releases all resources.
func (p *BatchingProducer) Dispose() {
	p.loopCancel()
	p.publisherChannel.broadcast()
	<-p.loopDone
}

// ProduceProcessInfo is the context for a Produce call
type ProduceProcessInfo struct {
	Finished   chan struct{}
	Err        error
	cancelFunc context.CancelFunc
	cancelled  chan struct{}
}

// IsCancelled returns true if the context has been cancelled
func (p *ProduceProcessInfo) IsCancelled() bool {
	select {
	case <-p.cancelled:
		return true
	default:
		return false
	}
}

// IsFinished returns true if the context has been finished
func (p *ProduceProcessInfo) IsFinished() bool {
	select {
	case <-p.Finished:
		return true
	default:
		return false
	}
}

// setErr sets the error for the ProduceProcessInfo
func (p *ProduceProcessInfo) setErr(err error) {
	p.Err = err
	p.cancelFunc()
	close(p.Finished)
}

// cancel the ProduceProcessInfo context
func (p *ProduceProcessInfo) cancel() {
	p.cancelFunc()
	close(p.Finished)
	close(p.cancelled)
}

// success closes the ProduceProcessInfo Finished channel
func (p *ProduceProcessInfo) success() {
	p.cancelFunc()
	close(p.Finished)
}

type produceProcessInfoKey struct{}

// GetProduceProcessInfo adds a new produce info to the BatchingProducer.Produce context
func (p *BatchingProducer) getProduceProcessInfo(ctx context.Context) *ProduceProcessInfo {
	return ctx.Value(produceProcessInfoKey{}).(*ProduceProcessInfo)
}

// Produce a message to producer queue. The return info can be used to wait for the message to be published.
func (p *BatchingProducer) Produce(ctx context.Context, message interface{}) (*ProduceProcessInfo, error) {
	ctx, cancel := context.WithCancel(ctx)
	info := &ProduceProcessInfo{
		Finished:   make(chan struct{}),
		cancelled:  make(chan struct{}),
		cancelFunc: cancel,
	}
	ctx = context.WithValue(ctx, produceProcessInfoKey{}, info)
	if !p.publisherChannel.tryWrite(produceMessage{
		message: message,
		ctx:     ctx,
	}) {
		if p.publisherChannel.isComplete() {
			return info, &InvalidOperationException{Topic: p.topic}
		}
		return info, &ProducerQueueFullException{topic: p.topic}
	}
	return info, nil
}

// publishLoop is the main loop of the producer. It reads messages from the queue and publishes them in batches.
func (p *BatchingProducer) publishLoop(ctx context.Context) {
	defer close(p.loopDone)

	plog.Debug("Producer is starting the publisher loop for topic", log.String("topic", p.topic))
	batchWrapper := newPubSubBatchWithReceipts()

	handleUnrecoverableError := func(err error) {
		p.stopAcceptingNewMessages()
		if p.config.LogThrottle() == actor.Open {
			plog.Error("Error in the publisher loop of Producer for topic", log.String("topic", p.topic), log.Error(err))
		}
		p.failBatch(batchWrapper, err)
		p.failPendingMessages(err)
	}

	_, err := p.publisher.Initialize(ctx, p.topic, PublisherConfig{IdleTimeout: p.config.PublisherIdleTimeout})
	if err != nil && err != context.Canceled {
		handleUnrecoverableError(err)
	}

loop:
	for {
		select {
		case <-ctx.Done():
			p.stopAcceptingNewMessages()
			break loop
		default:
			if msg, ok := p.publisherChannel.tryRead(); ok {

				// if msg ctx not done
				select {
				case <-msg.ctx.Done():
					p.getProduceProcessInfo(msg.ctx).cancel()
				default:
					batchWrapper.batch.Envelopes = append(batchWrapper.batch.Envelopes, msg.message)
					batchWrapper.ctxArr = append(batchWrapper.ctxArr, msg.ctx)
				}

				if len(batchWrapper.batch.Envelopes) < p.config.BatchSize {
					continue
				}

				err := p.publishBatch(ctx, batchWrapper)
				if err != nil {
					handleUnrecoverableError(err)
					break loop
				}
				batchWrapper = newPubSubBatchWithReceipts()
			} else {
				if len(batchWrapper.batch.Envelopes) > 0 {
					err := p.publishBatch(ctx, batchWrapper)
					if err != nil {
						handleUnrecoverableError(err)
						break loop
					}
					batchWrapper = newPubSubBatchWithReceipts()
				}
				p.publisherChannel.waitToRead()
			}
		}
	}
	p.cancelBatch(batchWrapper)
	p.cancelPendingMessages()
}

// cancelPendingMessages cancels all pending messages
func (p *BatchingProducer) cancelPendingMessages() {
	for {
		if msg, ok := p.publisherChannel.tryRead(); ok {
			p.getProduceProcessInfo(msg.ctx).cancel()
		} else {
			break
		}
	}
}

// cancelBatch cancels all contexts in the batch wrapper
func (p *BatchingProducer) cancelBatch(batchWrapper *pubsubBatchWithReceipts) {
	for _, ctx := range batchWrapper.ctxArr {
		p.getProduceProcessInfo(ctx).cancel()
	}

	// ensure once cancelled, we won't touch the batch anymore
	p.clearBatch(batchWrapper)
}

// failPendingMessages fails all pending messages
func (p *BatchingProducer) failPendingMessages(err error) {
	for {
		if msg, ok := p.publisherChannel.tryRead(); ok {
			p.getProduceProcessInfo(msg.ctx).setErr(err)
		} else {
			break
		}
	}
}

// failBatch marks all contexts in the batch wrapper as failed
func (p *BatchingProducer) failBatch(batchWrapper *pubsubBatchWithReceipts, err error) {
	for _, ctx := range batchWrapper.ctxArr {
		p.getProduceProcessInfo(ctx).setErr(err)
	}

	// ensure once failed, we won't touch the batch anymore
	p.clearBatch(batchWrapper)
}

// clearBatch clears the batch wrapper
func (p *BatchingProducer) clearBatch(batchWrapper *pubsubBatchWithReceipts) {
	batchWrapper.batch = &PubSubBatch{Envelopes: make([]interface{}, 0, 10)}
	batchWrapper.ctxArr = batchWrapper.ctxArr[:0]
}

// completeBatch marks all contexts in the batch wrapper as completed
func (p *BatchingProducer) completeBatch(batchWrapper *pubsubBatchWithReceipts) {
	for _, ctx := range batchWrapper.ctxArr {
		p.getProduceProcessInfo(ctx).success()
	}

	// ensure once completed, we won't touch the batch anymore
	p.clearBatch(batchWrapper)
}

// removeCancelledFromBatch removes all cancelled contexts from the batch wrapper
func (p *BatchingProducer) removeCancelledFromBatch(batchWrapper *pubsubBatchWithReceipts) {
	for i := len(batchWrapper.ctxArr) - 1; i >= 0; i-- {
		select {
		case <-batchWrapper.ctxArr[i].Done():
			info := p.getProduceProcessInfo(batchWrapper.ctxArr[i])
			select {
			case <-info.Finished:
				// if the message is already finished, we don't need to do anything
			default:
				info.cancel()
			}

			batchWrapper.batch.Envelopes = append(batchWrapper.batch.Envelopes[:i], batchWrapper.batch.Envelopes[i+1:]...)
			batchWrapper.ctxArr = append(batchWrapper.ctxArr[:i], batchWrapper.ctxArr[i+1:]...)
		default:
			continue
		}
	}
}

// stopAcceptingNewMessages stops accepting new messages into the channel.
func (p *BatchingProducer) stopAcceptingNewMessages() {
	p.publisherChannel.complete()
}

// publishBatch publishes a batch of messages using Publisher.
func (p *BatchingProducer) publishBatch(ctx context.Context, batchWrapper *pubsubBatchWithReceipts) error {
	retries := 0
	retry := true

loop:
	for retry {
		select {
		case <-ctx.Done():
			p.cancelBatch(batchWrapper)
			break loop
		default:
			retries++
			_, err := p.publisher.PublishBatch(ctx, p.topic, batchWrapper.batch, WithTimeout(p.config.PublishTimeout))
			if err != nil {
				decision := p.config.OnPublishingError(retries, err, batchWrapper.batch)
				if decision == FailBatchAndStop {
					p.stopAcceptingNewMessages()
					p.failBatch(batchWrapper, err)
					return err // let the main producer loop exit
				}

				if p.config.LogThrottle() == actor.Open {
					plog.Warn("Error while publishing batch", log.Error(err))
				}

				if decision == FailBatchAndContinue {
					p.failBatch(batchWrapper, err)
					return nil
				}

				// the decision is to retry
				// if any of the messages have been canceled in the meantime, remove them and cancel the delivery report
				p.removeCancelledFromBatch(batchWrapper)

				if len(batchWrapper.batch.Envelopes) == 0 {
					retry = false
				} else if decision.Delay > 0 {
					time.Sleep(decision.Delay)
				}

				continue
			}

			retry = false
			p.completeBatch(batchWrapper)
		}
	}

	return nil
}

type ProducerQueueFullException struct {
	topic string
}

func (p *ProducerQueueFullException) Error() string {
	return "Producer for topic " + p.topic + " has full queue"
}

func (p *ProducerQueueFullException) Is(target error) bool {
	_, ok := target.(*ProducerQueueFullException)
	return ok
}

type InvalidOperationException struct {
	Topic string
}

func (i *InvalidOperationException) Is(err error) bool {
	_, ok := err.(*InvalidOperationException)
	return ok
}

func (i *InvalidOperationException) Error() string {
	return "Producer for topic " + i.Topic + " is stopped, cannot produce more messages."
}

// channel is a wrapper around a channel that can be used to read and write messages.
// messages must be pointers.
type channel[T any] interface {
	tryWrite(msg T) bool
	tryRead() (T, bool)
	isComplete() bool
	complete()
	empty() bool
	waitToRead()
	broadcast()
}

// BoundedChannel is a bounded channel with the given capacity.
type boundedChannel[T any] struct {
	capacity int
	c        chan T
	quit     chan struct{}
	once     *sync.Once
	cond     *sync.Cond
	left     *atomic.Bool
}

func (b *boundedChannel[T]) tryWrite(msg T) bool {
	select {
	case b.c <- msg:
		b.cond.Broadcast()
		return true
	case <-b.quit:
		return false
	default:
		return false
	}
}

func (b *boundedChannel[T]) tryRead() (msg T, ok bool) {
	var msgDefault T
	select {
	case msg, ok = <-b.c:
		return
	default:
		return msgDefault, false
	}
}

func (b *boundedChannel[T]) isComplete() bool {
	select {
	case <-b.quit:
		return true
	default:
		return false
	}
}

func (b *boundedChannel[T]) complete() {
	b.once.Do(func() {
		close(b.quit)
	})
}

func (b *boundedChannel[T]) empty() bool {
	return len(b.c) == 0
}

func (b *boundedChannel[T]) waitToRead() {
	b.cond.L.Lock()
	defer b.cond.L.Unlock()
	for b.empty() && !b.left.Load() {
		b.cond.Wait()
	}
	b.left.Store(false)
}

func (b *boundedChannel[T]) broadcast() {
	b.left.Store(true)
	b.cond.Broadcast()
}

// newBoundedChannel creates a new bounded channel with the given capacity.
func newBoundedChannel[T any](capacity int) channel[T] {
	return &boundedChannel[T]{
		capacity: capacity,
		c:        make(chan T, capacity),
		quit:     make(chan struct{}),
		cond:     sync.NewCond(&sync.Mutex{}),
		once:     &sync.Once{},
		left:     &atomic.Bool{},
	}
}

// UnboundedChannel is an unbounded channel.
type unboundedChannel[T any] struct {
	queue *mpsc.Queue
	quit  chan struct{}
	once  *sync.Once
	cond  *sync.Cond
	left  *atomic.Bool
}

func (u *unboundedChannel[T]) tryWrite(msg T) bool {
	select {
	case <-u.quit:
		return false
	default:
		u.queue.Push(msg)
		u.cond.Broadcast()
		return true
	}
}

func (u *unboundedChannel[T]) tryRead() (T, bool) {
	var msg T
	tmp := u.queue.Pop()
	if tmp == nil {
		return msg, false
	} else {
		u.cond.Broadcast()
		return tmp.(T), true
	}
}

func (u *unboundedChannel[T]) complete() {
	u.once.Do(func() {
		close(u.quit)
	})
}

func (u *unboundedChannel[T]) isComplete() bool {
	select {
	case <-u.quit:
		return true
	default:
		return false
	}
}

func (u *unboundedChannel[T]) empty() bool {
	return u.queue.Empty()
}

func (u *unboundedChannel[T]) waitToRead() {
	u.cond.L.Lock()
	defer u.cond.L.Unlock()
	for u.empty() && !u.left.Load() {
		u.cond.Wait()
	}
	u.left.Store(false)
}

func (u *unboundedChannel[T]) broadcast() {
	u.left.Store(true)
	u.cond.Broadcast()
}

// newUnboundedChannel creates a new unbounded channel.
func newUnboundedChannel[T any]() channel[T] {
	return &unboundedChannel[T]{
		queue: mpsc.New(),
		quit:  make(chan struct{}),
		cond:  sync.NewCond(&sync.Mutex{}),
		once:  &sync.Once{},
		left:  &atomic.Bool{},
	}
}
