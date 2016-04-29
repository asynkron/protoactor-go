package actor

import (
	"runtime"
	"sync/atomic"

	"github.com/rogeralsing/gam/queue"
)

type UnboundedMailbox struct {
	throughput      int
	userMailbox     *queue.Queue
	systemMailbox   *queue.Queue
	schedulerStatus int32
	hasMoreMessages int32
	userInvoke      func(interface{})
	systemInvoke    func(SystemMessage)
}

func (mailbox *UnboundedMailbox) PostUserMessage(message interface{}) {
	mailbox.userMailbox.Push(message)
	mailbox.schedule()
}

func (mailbox *UnboundedMailbox) PostSystemMessage(message SystemMessage) {
	mailbox.systemMailbox.Push(message)
	mailbox.schedule()
}

func (mailbox *UnboundedMailbox) schedule() {
	atomic.StoreInt32(&mailbox.hasMoreMessages, MailboxHasMoreMessages) //we have more messages to process
	if atomic.CompareAndSwapInt32(&mailbox.schedulerStatus, MailboxIdle, MailboxRunning) {
		go mailbox.processMessages()
	}
}

func (mailbox *UnboundedMailbox) Suspend() {

}

func (mailbox *UnboundedMailbox) Resume() {

}

func (mailbox *UnboundedMailbox) processMessages() {
	//we are about to start processing messages, we can safely reset the message flag of the mailbox
	atomic.StoreInt32(&mailbox.hasMoreMessages, MailboxHasNoMessages)

	done := false
	for !done {
		//process x messages in sequence, then exit
		for i := 0; i < mailbox.throughput; i++ {
			if sysMsg, ok := mailbox.systemMailbox.Pop(); ok {
				sys, _ := sysMsg.(SystemMessage)
				mailbox.systemInvoke(sys)
			} else if userMsg, ok := mailbox.userMailbox.Pop(); ok {

				mailbox.userInvoke(userMsg)
			} else {
				done = true
				break
			}
		}
		runtime.Gosched()
	}

	//set mailbox to idle
	atomic.StoreInt32(&mailbox.schedulerStatus, MailboxIdle)
	//check if there are still messages to process (sent after the message loop ended)
	if atomic.SwapInt32(&mailbox.hasMoreMessages, MailboxHasNoMessages) == MailboxHasMoreMessages {
		mailbox.schedule()
	}

}

func NewUnboundedMailbox(throughput int) MailboxProducer {
	return func() Mailbox {
		userMailbox := queue.New()
		systemMailbox := queue.New()
		mailbox := UnboundedMailbox{
			throughput:      throughput,
			userMailbox:     userMailbox,
			systemMailbox:   systemMailbox,
			hasMoreMessages: MailboxHasNoMessages,
			schedulerStatus: MailboxIdle,
		}
		return &mailbox
	}
}

func (mailbox *UnboundedMailbox) RegisterHandlers(userInvoke func(interface{}), systemInvoke func(SystemMessage)) {
	mailbox.userInvoke = userInvoke
	mailbox.systemInvoke = systemInvoke
}
