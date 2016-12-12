package actor

import (
	"github.com/AsynkronIT/gam/actor/lfqueue"
	"github.com/Workiva/go-datastructures/queue"
)

type boundedMailboxQueue struct {
	userMailbox *queue.RingBuffer
}

func (q *boundedMailboxQueue) Push(m interface{}) {
	q.userMailbox.Put(m)
}

func (q *boundedMailboxQueue) Pop() interface{} {
	if q.userMailbox.Len() > 0 {
		m, _ := q.userMailbox.Get()
		return m
	}
	return nil
}

//NewUnboundedMailbox creates an unbounded mailbox
func NewBoundedMailbox(size int) MailboxProducer {
	return func() Mailbox {
		q := &boundedMailboxQueue{
			userMailbox: queue.NewRingBuffer(uint64(size)),
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
