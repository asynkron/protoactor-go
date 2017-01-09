package actor

import (
	"github.com/AsynkronIT/goring"
	"github.com/AsynkronIT/protoactor-go/internal/queue/mpsc"
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

var defaultMailboxProducer = NewUnboundedMailbox()

// NewUnboundedMailbox creates an unbounded mailbox
func NewUnboundedMailbox(mailboxStats ...MailboxStatistics) MailboxProducer {
	return func() Mailbox {
		q := &unboundedMailboxQueue{
			userMailbox: goring.New(10),
		}
		return &DefaultMailbox{
			systemMailbox: mpsc.New(),
			userMailbox:   q,
			mailboxStats:  mailboxStats,
		}
	}
}
