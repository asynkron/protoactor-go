package gam

import (
	"sync/atomic"
)

type BoundedMailbox struct {
	userMailbox     chan interface{}
	systemMailbox   chan SystemMessage
	schedulerStatus int32
	hasMoreMessages int32
	userInvoke      func(interface{})
	systemInvoke    func(SystemMessage)
}

func (mailbox *BoundedMailbox) PostUserMessage(message interface{}) {
	mailbox.userMailbox <- message
	mailbox.schedule()
}

func (mailbox *BoundedMailbox) PostSystemMessage(message SystemMessage) {
	mailbox.systemMailbox <- message
	mailbox.schedule()
}

func (mailbox *BoundedMailbox) schedule() {
	atomic.StoreInt32(&mailbox.hasMoreMessages, MailboxHasMoreMessages) //we have more messages to process
	if atomic.CompareAndSwapInt32(&mailbox.schedulerStatus, MailboxIdle, MailboxRunning) {
		go mailbox.processMessages()
	}
}

func (mailbox *BoundedMailbox) Suspend() {

}

func (mailbox *BoundedMailbox) Resume() {

}

func (mailbox *BoundedMailbox) processMessages() {
	//we are about to start processing messages, we can safely reset the message flag of the mailbox
	atomic.StoreInt32(&mailbox.hasMoreMessages, MailboxHasNoMessages)


	done := false
	//process x messages in sequence, then exit
	for i := 0; i < 30; i++ {
		select {
		case sysMsg := <-mailbox.systemMailbox:
			//prioritize system messages
			mailbox.systemInvoke(sysMsg)
		default:
			//if no system message is present, try read user message
			select {
			case userMsg := <-mailbox.userMailbox:
				mailbox.userInvoke(userMsg)
			default:
				done = true
				break
			}
		}
	}
	if !done {
		atomic.StoreInt32(&mailbox.hasMoreMessages, MailboxHasMoreMessages)
	}
	
	//set mailbox to idle
	atomic.StoreInt32(&mailbox.schedulerStatus, MailboxIdle)
	//check if there are still messages to process (sent after the message loop ended)
	if  atomic.SwapInt32(&mailbox.hasMoreMessages,MailboxHasNoMessages) == MailboxHasMoreMessages {
		mailbox.schedule()
	}
}

func NewBoundedMailbox(boundedSize int) Mailbox {
	userMailbox := make(chan interface{}, boundedSize)
	systemMailbox := make(chan SystemMessage, boundedSize)
	mailbox := BoundedMailbox{
		userMailbox:     userMailbox,
		systemMailbox:   systemMailbox,
		hasMoreMessages: MailboxHasNoMessages,
		schedulerStatus: MailboxIdle,
	}
	return &mailbox
}

func (mailbox *BoundedMailbox) RegisterHandlers(userInvoke func(interface{}), systemInvoke func(SystemMessage)) {
	mailbox.userInvoke = userInvoke
	mailbox.systemInvoke = systemInvoke
}
