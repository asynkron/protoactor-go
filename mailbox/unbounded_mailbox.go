package mailbox

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

// NewUnboundedMailbox creates an unbounded mailbox
func NewUnboundedProducer(mailboxStats ...Statistics) Producer {
	return func(invoker MessageInvoker, dispatcher Dispatcher) Inbound {
		q := &unboundedMailboxQueue{
			userMailbox: goring.New(10),
		}
		return &DefaultMailbox{
			systemMailbox: mpsc.New(),
			userMailbox:   q,
			invoker:       invoker,
			mailboxStats:  mailboxStats,
			dispatcher:    dispatcher,
		}
	}
}
