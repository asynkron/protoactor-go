package actor

import (
	"runtime"
	"sync/atomic"

	"github.com/AsynkronIT/gam/actor/lfqueue"
	"github.com/AsynkronIT/goring"
)

type unboundedMailbox struct {
	userMailbox     *goring.Queue
	systemMailbox   *lfqueue.LockfreeQueue
	schedulerStatus int32
	hasMoreMessages int32
	invoker         MessageInvoker
	dispatcher      Dispatcher
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
		mailbox.dispatcher.Schedule(mailbox.processMessages)
	}
}

func (mailbox *unboundedMailbox) Suspend() {

}

func (mailbox *unboundedMailbox) Resume() {

}

func (mailbox *unboundedMailbox) processMessages() {
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
			} else if userMsg, ok := mailbox.userMailbox.Pop(); ok {
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

//NewUnboundedMailbox creates an unbounded mailbox
func NewUnboundedMailbox() MailboxProducer {
	return func() Mailbox {
		userMailbox := goring.New(10)
		systemMailbox := lfqueue.NewLockfreeQueue()
		mailbox := unboundedMailbox{
			userMailbox:     userMailbox,
			systemMailbox:   systemMailbox,
			hasMoreMessages: mailboxHasNoMessages,
			schedulerStatus: mailboxIdle,
		}
		return &mailbox
	}
}

func (mailbox *unboundedMailbox) RegisterHandlers(invoker MessageInvoker, dispatcher Dispatcher) {
	mailbox.invoker = invoker
	mailbox.dispatcher = dispatcher
}
