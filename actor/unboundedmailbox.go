package actor

import "sync/atomic"
import "github.com/Workiva/go-datastructures/queue"
import _ "log"

type UnboundedMailbox struct {
	userMailbox     *queue.Queue
	systemMailbox   *queue.Queue
	schedulerStatus int32
	hasMoreMessages int32
	userInvoke      func(interface{})
	systemInvoke    func(SystemMessage)
}

func (mailbox *UnboundedMailbox) PostUserMessage(message interface{}) {
	mailbox.userMailbox.Put(message)
	mailbox.schedule()
}

func (mailbox *UnboundedMailbox) PostSystemMessage(message SystemMessage) {
	mailbox.systemMailbox.Put(message)
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
	//process x messages in sequence, then exit
	for i := 0; i < 30; i++ {
		if !mailbox.systemMailbox.Empty() {
			sysMsg, _ := mailbox.systemMailbox.Get(1)
			first := sysMsg[0].(SystemMessage)
			mailbox.systemInvoke(first)
		} else if !mailbox.userMailbox.Empty() {
			userMsg, _ := mailbox.userMailbox.Get(1)
			first := userMsg[0]
			mailbox.userInvoke(first)
		} else {
			done = true
			break
		}
	}

	if !done {
		atomic.StoreInt32(&mailbox.hasMoreMessages, MailboxHasMoreMessages)
	}

	//set mailbox to idle
	atomic.StoreInt32(&mailbox.schedulerStatus, MailboxIdle)
	//check if there are still messages to process (sent after the message loop ended)
	if atomic.SwapInt32(&mailbox.hasMoreMessages, MailboxHasNoMessages) == MailboxHasMoreMessages {
		mailbox.schedule()
	}

}

func NewUnboundedMailbox() Mailbox {
	userMailbox := queue.New(0)
	systemMailbox := queue.New(0)
	mailbox := UnboundedMailbox{
		userMailbox:     userMailbox,
		systemMailbox:   systemMailbox,
		hasMoreMessages: MailboxHasNoMessages,
		schedulerStatus: MailboxIdle,
	}
	return &mailbox
}

func (mailbox *UnboundedMailbox) RegisterHandlers(userInvoke func(interface{}), systemInvoke func(SystemMessage)) {
	mailbox.userInvoke = userInvoke
	mailbox.systemInvoke = systemInvoke
}
