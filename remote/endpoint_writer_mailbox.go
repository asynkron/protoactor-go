package remote

import (
	"log"
	"runtime"
	"sync/atomic"

	"github.com/AsynkronIT/goring"
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/internal/core"
	"github.com/AsynkronIT/protoactor-go/internal/queue/lfqueue"
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
	systemMailbox   *lfqueue.LockfreeQueue
	schedulerStatus int32
	hasMoreMessages int32
	invoker         actor.MessageInvoker
	batchSize       int
	dispatcher      actor.Dispatcher
	suspended       bool
}

func (m *endpointWriterMailbox) PostUserMessage(message interface{}) {
	//batching mailbox only use the message part
	m.userMailbox.Push(message)
	m.schedule()
}

func (m *endpointWriterMailbox) PostSystemMessage(message actor.SystemMessage) {
	m.systemMailbox.Push(message)
	m.schedule()
}

func (m *endpointWriterMailbox) schedule() {
	atomic.StoreInt32(&m.hasMoreMessages, mailboxHasMoreMessages) //we have more messages to process
	if atomic.CompareAndSwapInt32(&m.schedulerStatus, mailboxIdle, mailboxRunning) {
		m.dispatcher.Schedule(m.processMessages)
	}
}

func (m *endpointWriterMailbox) processMessages() {
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
}

func (m *endpointWriterMailbox) run() {
	var msg interface{}
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[ACTOR] '%v' Recovering from: %v. Detailed stack: %v", m.invoker, r, core.IdentifyPanic())
			m.invoker.EscalateFailure(nil, r, msg)
		}
	}()

	for {
		// keep processing system messages until queue is empty
		if msg = m.systemMailbox.Pop(); msg != nil {
			sys, _ := msg.(actor.SystemMessage)
			switch sys.(type) {
			case *actor.SuspendMailbox:
				m.suspended = true
			case *actor.ResumeMailbox:
				m.suspended = false
			}

			m.invoker.InvokeSystemMessage(sys)
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

func newEndpointWriterMailbox(batchSize, initialSize int) actor.MailboxProducer {

	return func() actor.Mailbox {
		userMailbox := goring.New(int64(initialSize))
		systemMailbox := lfqueue.NewLockfreeQueue()
		mailbox := endpointWriterMailbox{
			userMailbox:     userMailbox,
			systemMailbox:   systemMailbox,
			hasMoreMessages: mailboxHasNoMessages,
			schedulerStatus: mailboxIdle,
			batchSize:       batchSize,
		}
		return &mailbox
	}
}

func (m *endpointWriterMailbox) RegisterHandlers(invoker actor.MessageInvoker, dispatcher actor.Dispatcher) {
	m.invoker = invoker
	m.dispatcher = dispatcher
}
