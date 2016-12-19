package remoting

import (
	"sync/atomic"

	"github.com/AsynkronIT/gam/actor"
	"github.com/AsynkronIT/goring"

	"runtime"
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
	systemMailbox   *goring.Queue
	schedulerStatus int32
	hasMoreMessages int32
	invoker         actor.MessageInvoker
	batchSize       int
	dispatcher      actor.Dispatcher
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

func (mailbox *endpointWriterMailbox) processMessages() {
	//we are about to start processing messages, we can safely reset the message flag of the mailbox
	atomic.StoreInt32(&mailbox.hasMoreMessages, mailboxHasNoMessages)
	batchSize := mailbox.batchSize
process:
	for {
		if sysMsg, ok := mailbox.systemMailbox.Pop(); ok {
			sys := sysMsg.(actor.SystemMessage)
			mailbox.invoker.InvokeSystemMessage(sys)
		} else if userMsg, ok := mailbox.userMailbox.PopMany(int64(batchSize)); ok {
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
		systemMailbox := goring.New(10)
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
