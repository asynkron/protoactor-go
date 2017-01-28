package mailbox

import (
	"runtime"
	"sync/atomic"

	"github.com/AsynkronIT/protoactor-go/internal/queue/mpsc"
	"github.com/AsynkronIT/protoactor-go/log"
)

type Statistics interface {
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
}

// The Inbound interface is used to enqueue messages to the mailbox
type Inbound interface {
	PostUserMessage(message interface{})
	PostSystemMessage(message interface{})
	Start()
}

// Producer is a function which creates a new mailbox
type Producer func(invoker MessageInvoker, dispatcher Dispatcher) Inbound

const (
	idle int32 = iota
	running
)

const (
	hasNoMessages int32 = iota
	hasMoreMessages
)

type defaultMailbox struct {
	userMailbox     queue
	systemMailbox   *mpsc.Queue
	schedulerStatus int32
	hasMoreMessages int32
	invoker         MessageInvoker
	dispatcher      Dispatcher
	suspended       bool
	mailboxStats    []Statistics
}

func (m *defaultMailbox) PostUserMessage(message interface{}) {
	for _, ms := range m.mailboxStats {
		ms.MessagePosted(message)
	}
	m.userMailbox.Push(message)
	m.schedule()
}

func (m *defaultMailbox) PostSystemMessage(message interface{}) {
	m.systemMailbox.Push(message)
	m.schedule()
}

func (m *defaultMailbox) schedule() {
	atomic.StoreInt32(&m.hasMoreMessages, hasMoreMessages) //we have more messages to process
	if atomic.CompareAndSwapInt32(&m.schedulerStatus, idle, running) {
		m.dispatcher.Schedule(m.processMessages)
	}
}

func (m *defaultMailbox) processMessages() {
	//we are about to start processing messages, we can safely reset the message flag of the mailbox
	atomic.StoreInt32(&m.hasMoreMessages, hasNoMessages)

process:
	m.run()

	// set mailbox to idle
	atomic.StoreInt32(&m.schedulerStatus, idle)

	// check if there are still messages to process (sent after the message loop ended)
	if atomic.SwapInt32(&m.hasMoreMessages, hasNoMessages) == hasMoreMessages {
		// try setting the mailbox back to running
		if atomic.CompareAndSwapInt32(&m.schedulerStatus, idle, running) {
			goto process
		}
	}

	for _, ms := range m.mailboxStats {
		ms.MailboxEmpty()
	}
}

func (m *defaultMailbox) run() {
	var msg interface{}

	defer func() {
		if r := recover(); r != nil {
			//force the has more messages to be true.
			//if there was a lot of messages on the queue, and we exit here.
			//there will be messages left at the queue that are not scheduled
			atomic.SwapInt32(&m.hasMoreMessages, hasMoreMessages)
			plog.Debug("[ACTOR] Recovering", log.Object("actor", m.invoker), log.Object("reason", r), log.Stack())
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
			switch msg.(type) {
			case *SuspendMailbox:
				m.suspended = true
			case *ResumeMailbox:
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

		if msg = m.userMailbox.Pop(); msg != nil {
			m.invoker.InvokeUserMessage(msg)
			for _, ms := range m.mailboxStats {
				ms.MessageReceived(msg)
			}
		} else {
			return
		}
	}

}

func (m *defaultMailbox) Start() {
	for _, ms := range m.mailboxStats {
		ms.MailboxStarted()
	}
}
