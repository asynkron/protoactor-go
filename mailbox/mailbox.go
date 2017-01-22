package mailbox

import (
	"log"
	"runtime"
	"sync/atomic"

	"github.com/AsynkronIT/protoactor-go/internal/core"
	"github.com/AsynkronIT/protoactor-go/internal/queue/mpsc"
)

type Statistics interface {
	MailboxStarted()
	MessagePosted(message interface{})
	MessageReceived(message interface{})
	MailboxEmpty()
}

type MessageInvoker interface {
	InvokeSystemMessage(interface{})
	InvokeUserMessage(interface{})
	EscalateFailure(reason interface{}, message interface{})
}

type Inbound interface {
	PostUserMessage(message interface{})
	PostSystemMessage(message interface{})
	Start()
}

type Producer func(invoker MessageInvoker, dispatcher Dispatcher) Inbound

const (
	mailboxIdle int32 = iota
	mailboxRunning
)

const (
	mailboxHasNoMessages int32 = iota
	mailboxHasMoreMessages
)

type DefaultMailbox struct {
	userMailbox     MailboxQueue
	systemMailbox   *mpsc.Queue
	schedulerStatus int32
	hasMoreMessages int32
	invoker         MessageInvoker
	dispatcher      Dispatcher
	suspended       bool
	mailboxStats    []Statistics
}

func (m *DefaultMailbox) PostUserMessage(message interface{}) {
	for _, ms := range m.mailboxStats {
		ms.MessagePosted(message)
	}
	m.userMailbox.Push(message)
	m.schedule()
}

func (m *DefaultMailbox) PostSystemMessage(message interface{}) {
	m.systemMailbox.Push(message)
	m.schedule()
}

func (m *DefaultMailbox) schedule() {
	atomic.StoreInt32(&m.hasMoreMessages, mailboxHasMoreMessages) //we have more messages to process
	if atomic.CompareAndSwapInt32(&m.schedulerStatus, mailboxIdle, mailboxRunning) {
		m.dispatcher.Schedule(m.processMessages)
	}
}

func (m *DefaultMailbox) processMessages() {
	//we are about to start processing messages, we can safely reset the message flag of the mailbox
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

	for _, ms := range m.mailboxStats {
		ms.MailboxEmpty()
	}
}

func (m *DefaultMailbox) run() {
	var msg interface{}

	defer func() {
		if r := recover(); r != nil {
			log.Printf("[ACTOR] '%s' Recovering from: %v. Detailed stack: %v", m.invoker, r, core.IdentifyPanic())
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

func (m *DefaultMailbox) Start() {
	for _, ms := range m.mailboxStats {
		ms.MailboxStarted()
	}
}
