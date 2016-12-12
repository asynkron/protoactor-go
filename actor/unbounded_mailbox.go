package actor

import (
	"github.com/AsynkronIT/gam/actor/lfqueue"
	"github.com/AsynkronIT/goring"
)

type unboundedMailboxQueue struct {
	userMailbox *goring.Queue
}

func (q *unboundedMailboxQueue) Push(m interface{}) {
	q.userMailbox.Push(m)
}

func (q *unboundedMailboxQueue) Pop() interface{} {
	m, o := q.userMailbox.Pop()
	if o {
		return m
	}
	return nil
}

//NewUnboundedMailbox creates an unbounded mailbox
func NewUnboundedMailbox() MailboxProducer {
	return func() Mailbox {
		q := &unboundedMailboxQueue{
			userMailbox: goring.New(10),
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
