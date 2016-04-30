package actor

import (
	"runtime"
	"sync/atomic"

	"github.com/Workiva/go-datastructures/queue"
)

type BoundedBatchingMailbox struct {
	batchSize       int
	userMailbox     *queue.RingBuffer
	systemMailbox   *queue.RingBuffer
	schedulerStatus int32
	hasMoreMessages int32
	userInvoke      func(interface{})
	systemInvoke    func(SystemMessage)
}

func (mailbox *BoundedBatchingMailbox) PostUserMessage(message interface{}) {
	mailbox.userMailbox.Put(message)
	mailbox.schedule()
}

func (mailbox *BoundedBatchingMailbox) PostSystemMessage(message SystemMessage) {
	mailbox.systemMailbox.Put(message)
	mailbox.schedule()
}

func (mailbox *BoundedBatchingMailbox) schedule() {
	atomic.StoreInt32(&mailbox.hasMoreMessages, MailboxHasMoreMessages) //we have more messages to process
	if atomic.CompareAndSwapInt32(&mailbox.schedulerStatus, MailboxIdle, MailboxRunning) {
		go mailbox.processMessages()
	}
}

func (mailbox *BoundedBatchingMailbox) Suspend() {

}

func (mailbox *BoundedBatchingMailbox) Resume() {

}

func (mailbox *BoundedBatchingMailbox) processMessages() {
	//we are about to start processing messages, we can safely reset the message flag of the mailbox
	atomic.StoreInt32(&mailbox.hasMoreMessages, MailboxHasNoMessages)

	done := false
	for !done {
		//process x messages in sequence, then exit

		if mailbox.systemMailbox.Len() > 0 {
			sysMsg, _ := mailbox.systemMailbox.Get()
			sys, _ := sysMsg.(SystemMessage)
			mailbox.systemInvoke(sys)
		} else if mailbox.userMailbox.Len() > 0 {
			len := int(mailbox.userMailbox.Len())
			if len > mailbox.batchSize {
				len = mailbox.batchSize
			}
			batch := make([]interface{}, len)
			for i := 0; i < len; i++ {
				item, _ := mailbox.userMailbox.Get()
				batch[i] = item
			}

			mailbox.userInvoke(batch)
		} else {
			done = true
			break
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

func NewBoundedBatchingMailbox(batchSize int, size int) MailboxProducer {
	return func() Mailbox {
		userMailbox := queue.NewRingBuffer(uint64(size))
		systemMailbox := queue.NewRingBuffer(100)
		mailbox := BoundedBatchingMailbox{
			batchSize:       batchSize,
			userMailbox:     userMailbox,
			systemMailbox:   systemMailbox,
			hasMoreMessages: MailboxHasNoMessages,
			schedulerStatus: MailboxIdle,
		}
		return &mailbox
	}
}

func (mailbox *BoundedBatchingMailbox) RegisterHandlers(userInvoke func(interface{}), systemInvoke func(SystemMessage)) {
	mailbox.userInvoke = userInvoke
	mailbox.systemInvoke = systemInvoke
}
