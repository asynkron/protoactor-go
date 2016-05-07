package remoting

import "sync/atomic"

import (
	"github.com/rogeralsing/gam/actor"
	"github.com/rogeralsing/gam/queue"
)
import _ "log"
import "runtime"

const (
	mailboxIdle    int32 = iota
	mailboxRunning int32 = iota
)
const (
	mailboxHasNoMessages   int32 = iota
	mailboxHasMoreMessages int32 = iota
)

type unboundedBatchingMailbox struct {
	userMailbox     *queue.Queue
	systemMailbox   *queue.Queue
	schedulerStatus int32
	hasMoreMessages int32
	userInvoke      func(interface{})
	systemInvoke    func(actor.SystemMessage)
	batchSize       int
}

func (mailbox *unboundedBatchingMailbox) PostUserMessage(message interface{}) {
	mailbox.userMailbox.Push(message)
	mailbox.schedule()
}

func (mailbox *unboundedBatchingMailbox) PostSystemMessage(message actor.SystemMessage) {
	mailbox.systemMailbox.Push(message)
	mailbox.schedule()
}

func (mailbox *unboundedBatchingMailbox) schedule() {
	atomic.StoreInt32(&mailbox.hasMoreMessages, mailboxHasMoreMessages) //we have more messages to process
	if atomic.CompareAndSwapInt32(&mailbox.schedulerStatus, mailboxIdle, mailboxRunning) {
		go mailbox.processMessages()
	}
}

func (mailbox *unboundedBatchingMailbox) Suspend() {

}

func (mailbox *unboundedBatchingMailbox) Resume() {

}

func (mailbox *unboundedBatchingMailbox) processMessages() {
	//we are about to start processing messages, we can safely reset the message flag of the mailbox
	atomic.StoreInt32(&mailbox.hasMoreMessages, mailboxHasNoMessages)
	batchSize := mailbox.batchSize
	done := false

	for !done {
		if sysMsg, ok := mailbox.systemMailbox.Pop(); ok {

			first := sysMsg.(actor.SystemMessage)
			mailbox.systemInvoke(first)
		} else if userMsg, ok := mailbox.userMailbox.PopMany(batchSize); ok {

			mailbox.userInvoke(userMsg)
		} else {
			done = true
			break
		}
		runtime.Gosched()
	}

	//set mailbox to idle
	atomic.StoreInt32(&mailbox.schedulerStatus, mailboxIdle)
	//check if there are still messages to process (sent after the message loop ended)
	if atomic.SwapInt32(&mailbox.hasMoreMessages, mailboxHasNoMessages) == mailboxHasMoreMessages {
		mailbox.schedule()
	}

}

func newUnboundedBatchingMailbox(batchSize, initialSize int) actor.MailboxProducer {

	return func() actor.Mailbox {
		userMailbox := queue.New(initialSize)
		systemMailbox := queue.New(10)
		mailbox := unboundedBatchingMailbox{
			userMailbox:     userMailbox,
			systemMailbox:   systemMailbox,
			hasMoreMessages: mailboxHasNoMessages,
			schedulerStatus: mailboxIdle,
			batchSize:       batchSize,
		}
		return &mailbox
	}
}

func (mailbox *unboundedBatchingMailbox) RegisterHandlers(userInvoke func(interface{}), systemInvoke func(actor.SystemMessage)) {
	mailbox.userInvoke = userInvoke
	mailbox.systemInvoke = systemInvoke
}
