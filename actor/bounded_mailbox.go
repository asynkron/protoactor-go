package actor

import (
	"github.com/AsynkronIT/protoactor-go/internal/queue/mpsc"
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

// NewBoundedMailbox creates an unbounded mailbox
func NewBoundedMailbox(size int, mailboxStats ...MailboxStatistics) MailboxProducer {
	return func() Mailbox {
		q := &boundedMailboxQueue{
			userMailbox: queue.NewRingBuffer(uint64(size)),
		}
		return &DefaultMailbox{
			systemMailbox: mpsc.New(),
			userMailbox:   q,
			mailboxStats:  mailboxStats,
		}
	}
}
