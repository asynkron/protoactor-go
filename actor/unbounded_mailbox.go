package actor

import (
	"runtime"
	"sync/atomic"

	"github.com/AsynkronIT/goring"
)

type unboundedMailbox struct {
	repeat          int
	throughput      int
	userMailbox     *goring.Queue
	systemMailbox   *goring.Queue
	schedulerStatus int32
	hasMoreMessages int32
	userInvoke      func(interface{})
	systemInvoke    func(SystemMessage)
}

func (mailbox *unboundedMailbox) PostUserMessage(message interface{}) {
	mailbox.userMailbox.Push(message)
	mailbox.schedule()
}

func (mailbox *unboundedMailbox) PostSystemMessage(message SystemMessage) {
	mailbox.systemMailbox.Push(message)
	mailbox.schedule()
}

func (mailbox *unboundedMailbox) schedule() {
	atomic.StoreInt32(&mailbox.hasMoreMessages, mailboxHasMoreMessages) //we have more messages to process
	if atomic.CompareAndSwapInt32(&mailbox.schedulerStatus, mailboxIdle, mailboxRunning) {
		go mailbox.processMessages()
	}
}

func (mailbox *unboundedMailbox) Suspend() {

}

func (mailbox *unboundedMailbox) Resume() {

}

func (mailbox *unboundedMailbox) processMessages() {
	//we are about to start processing messages, we can safely reset the message flag of the mailbox
	atomic.StoreInt32(&mailbox.hasMoreMessages, mailboxHasNoMessages)

	done := false
	for !done {
		//process x messages in sequence, then exit
		for i := 0; i < mailbox.throughput; i++ {
			if sysMsg, ok := mailbox.systemMailbox.Pop(); ok {
				sys, _ := sysMsg.(SystemMessage)
				mailbox.systemInvoke(sys)
			} else if userMsg, ok := mailbox.userMailbox.Pop(); ok {
				mailbox.userInvoke(userMsg.(interface{}))
			} else {
				done = true
				break
			}
		}
		if !done {
			runtime.Gosched()
		}
	}

	//set mailbox to idle
	atomic.StoreInt32(&mailbox.schedulerStatus, mailboxIdle)
	//check if there are still messages to process (sent after the message loop ended)
	if atomic.SwapInt32(&mailbox.hasMoreMessages, mailboxHasNoMessages) == mailboxHasMoreMessages {
		mailbox.schedule()
	}

}

//NewUnboundedMailbox creates an unbounded mailbox
func NewUnboundedMailbox(throughput int) MailboxProducer {
	return func() Mailbox {
		userMailbox := goring.New(10)
		systemMailbox := goring.New(10)
		mailbox := unboundedMailbox{
			repeat:          1,
			throughput:      throughput,
			userMailbox:     userMailbox,
			systemMailbox:   systemMailbox,
			hasMoreMessages: mailboxHasNoMessages,
			schedulerStatus: mailboxIdle,
		}
		return &mailbox
	}
}

func (mailbox *unboundedMailbox) RegisterHandlers(userInvoke func(interface{}), systemInvoke func(SystemMessage)) {
	mailbox.userInvoke = userInvoke
	mailbox.systemInvoke = systemInvoke
}
