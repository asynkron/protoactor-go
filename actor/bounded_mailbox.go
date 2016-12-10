package actor

import (
	"runtime"
	"sync/atomic"

	"github.com/AsynkronIT/gam/actor/lfqueue"
	"github.com/Workiva/go-datastructures/queue"
)

type boundedMailbox struct {
	userMailbox     *queue.RingBuffer
	systemMailbox   *lfqueue.LockfreeQueue
	schedulerStatus int32
	hasMoreMessages int32
	invoker         MessageInvoker
	dispatcher      Dispatcher
}

func (mailbox *boundedMailbox) PostUserMessage(message interface{}) {
	mailbox.userMailbox.Put(message)
	mailbox.schedule()
}

func (mailbox *boundedMailbox) PostSystemMessage(message SystemMessage) {
	mailbox.systemMailbox.Push(message)
	mailbox.schedule()
}

func (mailbox *boundedMailbox) schedule() {
	atomic.StoreInt32(&mailbox.hasMoreMessages, mailboxHasMoreMessages) //we have more messages to process
	if atomic.CompareAndSwapInt32(&mailbox.schedulerStatus, mailboxIdle, mailboxRunning) {
		mailbox.dispatcher.Schedule(mailbox.processMessages)
	}
}

func (mailbox *boundedMailbox) Suspend() {

}

func (mailbox *boundedMailbox) Resume() {

}

func (mailbox *boundedMailbox) processMessages() {
	//we are about to start processing messages, we can safely reset the message flag of the mailbox
	atomic.StoreInt32(&mailbox.hasMoreMessages, mailboxHasNoMessages)
	t := mailbox.dispatcher.Throughput()
	done := false
	for !done {
		//process x messages in sequence, then exit
		for i := 0; i < t; i++ {
			if sysMsg := mailbox.systemMailbox.Pop(); sysMsg != nil {
				sys, _ := sysMsg.(SystemMessage)
				mailbox.invoker.InvokeSystemMessage(sys)
			} else if mailbox.userMailbox.Len() > 0 {
				userMsg, _ := mailbox.userMailbox.Get()
				mailbox.invoker.InvokeUserMessage(userMsg)
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

func NewBoundedMailbox(size int) MailboxProducer {
	return func() Mailbox {
		userMailbox := queue.NewRingBuffer(uint64(size))
		systemMailbox := lfqueue.NewLockfreeQueue()
		mailbox := boundedMailbox{
			userMailbox:     userMailbox,
			systemMailbox:   systemMailbox,
			hasMoreMessages: mailboxHasNoMessages,
			schedulerStatus: mailboxIdle,
		}
		return &mailbox
	}
}

func (mailbox *boundedMailbox) RegisterHandlers(invoker MessageInvoker, dispatcher Dispatcher) {
	mailbox.invoker = invoker
	mailbox.dispatcher = dispatcher
}
