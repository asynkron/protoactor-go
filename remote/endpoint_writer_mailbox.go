package remote

import (
	"runtime"
	"sync/atomic"

	"github.com/otherview/protoactor-go/internal/queue/goring"
	"github.com/otherview/protoactor-go/internal/queue/mpsc"
	"github.com/otherview/protoactor-go/log"
	"github.com/otherview/protoactor-go/mailbox"
)

const (
	mailboxIdle    int32 = iota
	mailboxRunning int32 = iota
)
const (
	mailboxHasNoMessages   int32 = iota
	mailboxHasMoreMessages int32 = iota
)

type endpointWriterMailbox struct {
	userMailbox     *goring.Queue
	systemMailbox   *mpsc.Queue
	schedulerStatus int32
	hasMoreMessages int32
	invoker         mailbox.MessageInvoker
	batchSize       int
	dispatcher      mailbox.Dispatcher
	suspended       bool
}

func (m *endpointWriterMailbox) PostUserMessage(message interface{}) {
	// batching mailbox only use the message part
	m.userMailbox.Push(message)
	m.schedule()
}

func (m *endpointWriterMailbox) PostSystemMessage(message interface{}) {
	m.systemMailbox.Push(message)
	m.schedule()
}

func (m *endpointWriterMailbox) RegisterHandlers(invoker mailbox.MessageInvoker, dispatcher mailbox.Dispatcher) {
	m.invoker = invoker
	m.dispatcher = dispatcher
}

func (m *endpointWriterMailbox) Start() {
}

func (m *endpointWriterMailbox) schedule() {
	atomic.StoreInt32(&m.hasMoreMessages, mailboxHasMoreMessages) // we have more messages to process
	if atomic.CompareAndSwapInt32(&m.schedulerStatus, mailboxIdle, mailboxRunning) {
		m.dispatcher.Schedule(m.processMessages)
	}
}

func (m *endpointWriterMailbox) processMessages() {
	// we are about to start processing messages, we can safely reset the message flag of the mailbox
	atomic.StoreInt32(&m.hasMoreMessages, mailboxHasNoMessages)
process:
	m.run()

	// set mailbox to idle
	atomic.StoreInt32(&m.schedulerStatus, mailboxIdle)

	// check if there are still messages to process (sent after the message loop ended)
	if atomic.SwapInt32(&m.hasMoreMessages, mailboxHasNoMessages) == mailboxHasMoreMessages {
		// try setting the mailbox back to running
		if atomic.CompareAndSwapInt32(&m.schedulerStatus, mailboxIdle, mailboxRunning) {
			goto process
		}
	}
}

func (m *endpointWriterMailbox) run() {
	var msg interface{}
	defer func() {
		if r := recover(); r != nil {
			plog.Debug("[ACTOR] Recovering", log.Object("actor", m.invoker), log.Object("reason", r), log.Stack())
			m.invoker.EscalateFailure(r, msg)
		}
	}()

	for {
		// keep processing system messages until queue is empty
		if msg = m.systemMailbox.Pop(); msg != nil {
			switch msg.(type) {
			case *mailbox.SuspendMailbox:
				m.suspended = true
			case *mailbox.ResumeMailbox:
				m.suspended = false
			default:
				m.invoker.InvokeSystemMessage(msg)
			}

			continue
		}

		// didn't process a system message, so break until we are resumed
		if m.suspended {
			return
		}

		var ok bool
		if msg, ok = m.userMailbox.PopMany(int64(m.batchSize)); ok {
			m.invoker.InvokeUserMessage(msg)
		} else {
			return
		}

		runtime.Gosched()
	}
}

func endpointWriterMailboxProducer(batchSize, initialSize int) mailbox.Producer {
	return func() mailbox.Mailbox {
		userMailbox := goring.New(int64(initialSize))
		systemMailbox := mpsc.New()
		return &endpointWriterMailbox{
			userMailbox:     userMailbox,
			systemMailbox:   systemMailbox,
			hasMoreMessages: mailboxHasNoMessages,
			schedulerStatus: mailboxIdle,
			batchSize:       batchSize,
		}
	}
}
