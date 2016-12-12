package actor

import "github.com/AsynkronIT/gam/actor/lfqueue"

type unboundedLockfreeMailboxQueue struct {
	userMailbox *lfqueue.LockfreeQueue
}

func (q *unboundedLockfreeMailboxQueue) Push(m interface{}) {
	q.userMailbox.Push(m)
}

func (q *unboundedLockfreeMailboxQueue) Pop() interface{} {
	m := q.userMailbox.Pop()
	return m
}

//NewUnboundedMailbox creates an unbounded mailbox
func NewUnboundedLockfreeMailbox() MailboxProducer {
	return func() Mailbox {
		q := &unboundedLockfreeMailboxQueue{
			userMailbox: lfqueue.NewLockfreeQueue(),
		}
		systemMailbox := lfqueue.NewLockfreeQueue()
		mailbox := DefaultMailbox{
			hasMoreMessages: mailboxHasNoMessages,
			schedulerStatus: mailboxIdle,
			systemMailbox:   systemMailbox,
			userMailbox:     q,
		}

		return &mailbox
	}
}
