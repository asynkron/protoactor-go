package actor

import (
	"runtime"
	"sync/atomic"

	"github.com/asynkron/protoactor-go/internal/queue/mpsc"
	"github.com/asynkron/protoactor-go/log"
)

// MailboxMiddleware is an interface for intercepting messages and events in the mailbox
type MailboxMiddleware interface {
	MailboxStarted()
	MessagePosted(message interface{})
	MessageReceived(message interface{})
	MailboxEmpty()
}

// MessageInvoker is the interface used by a mailbox to forward messages for processing
type MessageInvoker interface {
	InvokeSystemMessage(interface{})
	InvokeUserMessage(interface{})
	EscalateFailure(reason interface{}, message interface{})
	cancelContext()
}

// Mailbox interface is used to enqueue messages to the mailbox
type Mailbox interface {
	PostUserMessage(message interface{})
	PostSystemMessage(message interface{})
	RegisterHandlers(invoker MessageInvoker, dispatcher Dispatcher)
	Start()
	UserMessageCount() int
}

// MailboxProducer is a function which creates a new mailbox
type MailboxProducer func() Mailbox

const (
	idle int32 = iota
	running
)

type defaultMailbox struct {
	userMailbox     queue
	systemMailbox   *mpsc.Queue
	schedulerStatus int32
	userMessages    int32
	sysMessages     int32
	suspended       int32
	invoker         MessageInvoker
	dispatcher      Dispatcher
	middlewares     []MailboxMiddleware
}

func (m *defaultMailbox) PostUserMessage(message interface{}) {
	// is it a raw batch message?
	if batch, ok := message.(MessageBatch); ok {
		messages := batch.GetMessages()

		for _, msg := range messages {
			m.PostUserMessage(msg)
		}
	}

	// is it an envelope batch message?
	// FIXME: check if this is still needed, maybe MessageEnvelope can only exist as a pointer
	if env, ok := message.(MessageEnvelope); ok {
		if batch, ok := env.Message.(MessageBatch); ok {
			messages := batch.GetMessages()

			for _, msg := range messages {
				m.PostUserMessage(msg)
			}
		}
	}
	if env, ok := message.(*MessageEnvelope); ok {
		if batch, ok := env.Message.(MessageBatch); ok {
			messages := batch.GetMessages()

			for _, msg := range messages {
				m.PostUserMessage(msg)
			}
		}
	}

	// normal messages
	for _, ms := range m.middlewares {
		ms.MessagePosted(message)
	}
	m.userMailbox.Push(message)
	atomic.AddInt32(&m.userMessages, 1)
	m.schedule()
}

func (m *defaultMailbox) PostSystemMessage(message interface{}) {
	for _, ms := range m.middlewares {
		ms.MessagePosted(message)
	}
	m.systemMailbox.Push(message)
	atomic.AddInt32(&m.sysMessages, 1)
	// check if message is stop
	if _, ok := message.(*Stop); ok {
		m.invoker.cancelContext()
	}

	m.schedule()
}

func (m *defaultMailbox) RegisterHandlers(invoker MessageInvoker, dispatcher Dispatcher) {
	m.invoker = invoker
	m.dispatcher = dispatcher
}

func (m *defaultMailbox) schedule() {
	if atomic.CompareAndSwapInt32(&m.schedulerStatus, idle, running) {
		m.dispatcher.Schedule(m.processMessages)
	}
}

func (m *defaultMailbox) processMessages() {
process:
	m.run()

	// set mailbox to idle
	atomic.StoreInt32(&m.schedulerStatus, idle)
	sys := atomic.LoadInt32(&m.sysMessages)
	user := atomic.LoadInt32(&m.userMessages)
	// check if there are still messages to process (sent after the message loop ended)
	if sys > 0 || (atomic.LoadInt32(&m.suspended) == 0 && user > 0) {
		// try setting the mailbox back to running
		if atomic.CompareAndSwapInt32(&m.schedulerStatus, idle, running) {
			//	fmt.Printf("looping %v %v %v\n", sys, user, m.suspended)
			goto process
		}
	}

	for _, ms := range m.middlewares {
		ms.MailboxEmpty()
	}
}

func (m *defaultMailbox) run() {
	var msg interface{}

	defer func() {
		if r := recover(); r != nil {
			plog.Info("[ACTOR] Recovering", log.Object("actor", m.invoker), log.Object("reason", r), log.Stack())
			m.invoker.EscalateFailure(r, msg)
		}
	}()

	i, t := 0, m.dispatcher.Throughput()
	for {
		if i > t {
			i = 0
			runtime.Gosched()
		}

		i++

		// keep processing system messages until queue is empty
		if msg = m.systemMailbox.Pop(); msg != nil {
			atomic.AddInt32(&m.sysMessages, -1)
			switch msg.(type) {
			case *SuspendMailbox:
				atomic.StoreInt32(&m.suspended, 1)
			case *ResumeMailbox:
				atomic.StoreInt32(&m.suspended, 0)
			default:
				m.invoker.InvokeSystemMessage(msg)
			}
			for _, ms := range m.middlewares {
				ms.MessageReceived(msg)
			}
			continue
		}

		// didn't process a system message, so break until we are resumed
		if atomic.LoadInt32(&m.suspended) == 1 {
			return
		}

		if msg = m.userMailbox.Pop(); msg != nil {
			atomic.AddInt32(&m.userMessages, -1)
			m.invoker.InvokeUserMessage(msg)
			for _, ms := range m.middlewares {
				ms.MessageReceived(msg)
			}
		} else {
			return
		}
	}
}

func (m *defaultMailbox) Start() {
	for _, ms := range m.middlewares {
		ms.MailboxStarted()
	}
}

func (m *defaultMailbox) UserMessageCount() int {
	return int(atomic.LoadInt32(&m.userMessages))
}
