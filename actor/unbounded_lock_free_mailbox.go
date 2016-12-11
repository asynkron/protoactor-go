package actor

import (
	"runtime"
	"sync/atomic"

	"github.com/AsynkronIT/gam/actor/lfqueue"
)

type unboundedLockfreeMailbox struct {
	userMailbox     *lfqueue.LockfreeQueue
	systemMailbox   *lfqueue.LockfreeQueue
	schedulerStatus int32
	hasMoreMessages int32
	invoker         MessageInvoker
	dispatcher      Dispatcher
}

func (mailbox *unboundedLockfreeMailbox) PostUserMessage(message interface{}) {
	mailbox.userMailbox.Push(message)
	mailbox.schedule()
}

func (mailbox *unboundedLockfreeMailbox) PostSystemMessage(message SystemMessage) {
	mailbox.systemMailbox.Push(message)
	mailbox.schedule()
}

func (mailbox *unboundedLockfreeMailbox) schedule() {
	atomic.StoreInt32(&mailbox.hasMoreMessages, mailboxHasMoreMessages) //we have more messages to process
	if atomic.CompareAndSwapInt32(&mailbox.schedulerStatus, mailboxIdle, mailboxRunning) {
		mailbox.dispatcher.Schedule(mailbox.processMessages)
	}
}

func (mailbox *unboundedLockfreeMailbox) processMessages() {
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
			} else if userMsg := mailbox.userMailbox.Pop(); userMsg != nil {
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

//NewUnboundedLockfreeMailbox creates an unbounded mailbox
func NewUnboundedLockfreeMailbox() MailboxProducer {
	return func() Mailbox {
		userMailbox := lfqueue.NewLockfreeQueue()
		systemMailbox := lfqueue.NewLockfreeQueue()
		mailbox := unboundedLockfreeMailbox{
			userMailbox:     userMailbox,
			systemMailbox:   systemMailbox,
			hasMoreMessages: mailboxHasNoMessages,
			schedulerStatus: mailboxIdle,
		}
		return &mailbox
	}
}

func (mailbox *unboundedLockfreeMailbox) RegisterHandlers(invoker MessageInvoker, dispatcher Dispatcher) {
	mailbox.invoker = invoker
	mailbox.dispatcher = dispatcher
}
