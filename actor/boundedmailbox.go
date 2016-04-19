package actor
import "github.com/rogeralsing/goactor/interfaces"

import (
	"sync/atomic"
)

type BoundedMailbox struct {
	userMailbox     chan interface{}
	systemMailbox   chan interfaces.SystemMessage
	schedulerStatus int32
	hasMoreMessages int32
	userInvoke      func(interface{})
	systemInvoke    func(interfaces.SystemMessage)
}

func (mailbox *BoundedMailbox) PostUserMessage(message interface{}) {
	mailbox.userMailbox <- message
	mailbox.schedule()
}

func (mailbox *BoundedMailbox) PostSystemMessage(message interfaces.SystemMessage) {
	mailbox.systemMailbox <- message
	mailbox.schedule()
}

func (mailbox *BoundedMailbox) schedule() {
	swapped := atomic.CompareAndSwapInt32(&mailbox.schedulerStatus, MailboxIdle, MailboxRunning)
	atomic.StoreInt32(&mailbox.hasMoreMessages, MailboxHasMoreMessages) //we have more messages to process
	if swapped {
		go mailbox.processMessages()
	}
}

func (mailbox *BoundedMailbox) Suspend(){
	
}

func (mailbox *BoundedMailbox) Resume(){
	
}

func (mailbox *BoundedMailbox) processMessages() {
	//we are about to start processing messages, we can safely reset the message flag of the mailbox
	atomic.StoreInt32(&mailbox.hasMoreMessages, MailboxHasNoMessages)

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
			}
		}
	}
	//set mailbox to idle
	atomic.StoreInt32(&mailbox.schedulerStatus, MailboxIdle)
	//check if there are still messages to process (sent after the message loop ended)
	hasMore := atomic.LoadInt32(&mailbox.hasMoreMessages)
	//what is the current status of the mailbox? it could have changed concurrently since the last two lines
	status := atomic.LoadInt32(&mailbox.schedulerStatus)
	//if there are still messages to process and the mailbox is idle, then reschedule a mailbox run
	//otherwise, we either exit, or the mailbox have been scheduled already by the schedule method
	if hasMore == MailboxHasMoreMessages && status == MailboxIdle {
		swapped := atomic.CompareAndSwapInt32(&mailbox.schedulerStatus, MailboxIdle, MailboxRunning)
		if swapped {
			go mailbox.processMessages()
		}
	}
}

func NewBoundedMailbox(userInvoke func(interface{}), systemInvoke func(interfaces.SystemMessage)) interfaces.Mailbox {
	userMailbox := make(chan interface{}, 100)
	systemMailbox := make(chan interfaces.SystemMessage, 100)
	mailbox := BoundedMailbox{
		userMailbox:     userMailbox,
		systemMailbox:   systemMailbox,
		hasMoreMessages: MailboxHasNoMessages,
		schedulerStatus: MailboxIdle,
		userInvoke:      userInvoke,
		systemInvoke:    systemInvoke,
	}
	return &mailbox
}
