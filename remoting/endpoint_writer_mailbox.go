package remoting

import (
	"runtime"
	"sync/atomic"

	"github.com/AsynkronIT/goring"
	"github.com/AsynkronIT/protoactor-go/actor"
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

func (mailbox *endpointWriterMailbox) PostUserMessage(message interface{}) {
	//batching mailbox only use the message part
	mailbox.userMailbox.Push(message)
	mailbox.schedule()
}

func (mailbox *endpointWriterMailbox) PostSystemMessage(message actor.SystemMessage) {
	mailbox.systemMailbox.Push(message)
	mailbox.schedule()
}

func (mailbox *endpointWriterMailbox) schedule() {
	atomic.StoreInt32(&mailbox.hasMoreMessages, mailboxHasMoreMessages) //we have more messages to process
	if atomic.CompareAndSwapInt32(&mailbox.schedulerStatus, mailboxIdle, mailboxRunning) {
		mailbox.dispatcher.Schedule(mailbox.processMessages)
	}
}

func (mailbox *endpointWriterMailbox) Suspend() {

}

func (mailbox *endpointWriterMailbox) Resume() {

}

func (m *endpointWriterMailbox) ConsumeSystemMessages() bool {
	if sysMsg := m.systemMailbox.Pop(); sysMsg != nil {
		sys, _ := sysMsg.(actor.SystemMessage)
		switch sys.(type) {
		case *actor.SuspendMailbox:
			m.suspended = true
		case *actor.ResumeMailbox:
			m.suspended = false
		}

		m.invoker.InvokeSystemMessage(sys)
		return true
	}
	return false
}

func (mailbox *endpointWriterMailbox) processMessages() {
	//we are about to start processing messages, we can safely reset the message flag of the mailbox
	atomic.StoreInt32(&mailbox.hasMoreMessages, mailboxHasNoMessages)
	batchSize := mailbox.batchSize
process:
	for {
		if mailbox.ConsumeSystemMessages() {
			continue
		} else if mailbox.suspended {
			// exit processing is suspended and no system messages were processed
			break process
		}

		if userMsg, ok := mailbox.userMailbox.PopMany(int64(batchSize)); ok {
			mailbox.invoker.InvokeUserMessage(userMsg)
		} else {
			break process
		}

		runtime.Gosched()
	}

	// set mailbox to idle
	atomic.StoreInt32(&mailbox.schedulerStatus, mailboxIdle)

	// check if there are still messages to process (sent after the message loop ended)
	if atomic.SwapInt32(&mailbox.hasMoreMessages, mailboxHasNoMessages) == mailboxHasMoreMessages {
		// try setting the mailbox back to running
		if atomic.CompareAndSwapInt32(&mailbox.schedulerStatus, mailboxIdle, mailboxRunning) {
			goto process
		}
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

func (mailbox *endpointWriterMailbox) RegisterHandlers(invoker actor.MessageInvoker, dispatcher actor.Dispatcher) {
	mailbox.invoker = invoker
	mailbox.dispatcher = dispatcher
}
