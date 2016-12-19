package actor

import (
	"runtime"
	"sync/atomic"

	"github.com/AsynkronIT/gam/actor/lfqueue"
)

type ReceiveUserMessage func(interface{})
type ReceiveSystemMessage func(SystemMessage)

type MailboxRunner func()
type MailboxProducer func() Mailbox
type Mailbox interface {
	PostUserMessage(message interface{})
	PostSystemMessage(message SystemMessage)
	RegisterHandlers(invoker MessageInvoker, dispatcher Dispatcher)
}

const (
	mailboxIdle    int32 = iota
	mailboxRunning int32 = iota
)
const (
	mailboxHasNoMessages   int32 = iota
	mailboxHasMoreMessages int32 = iota
)

type DefaultMailbox struct {
	userMailbox     MailboxQueue
	systemMailbox   *lfqueue.LockfreeQueue
	schedulerStatus int32
	hasMoreMessages int32
	invoker         MessageInvoker
	dispatcher      Dispatcher
	suspended       bool
}

func (m *DefaultMailbox) ConsumeSystemMessages() bool {
	if sysMsg := m.systemMailbox.Pop(); sysMsg != nil {
		sys, _ := sysMsg.(SystemMessage)
		switch sys.(type) {
		case *SuspendMailbox:
			m.suspended = true
		case *ResumeMailbox:
			m.suspended = false
		}

		m.invoker.InvokeSystemMessage(sys)
		return true
	}
	return false
}

func (m *DefaultMailbox) PostUserMessage(message interface{}) {
	m.userMailbox.Push(message)
	m.schedule()
}

func (m *DefaultMailbox) PostSystemMessage(message SystemMessage) {
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
	i, t := 0, m.dispatcher.Throughput()
process:
	for {
		if i > t {
			i = 0
			runtime.Gosched()
		}

		i++

		if m.ConsumeSystemMessages() {
			continue
		} else if m.suspended {
			// exit processing is suspended and no system messages were processed
			break process
		}

		if userMsg := m.userMailbox.Pop(); userMsg != nil {
			m.invoker.InvokeUserMessage(userMsg)
		} else {
			break process
		}
	}

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

func (mailbox *DefaultMailbox) RegisterHandlers(invoker MessageInvoker, dispatcher Dispatcher) {
	mailbox.invoker = invoker
	mailbox.dispatcher = dispatcher
}
