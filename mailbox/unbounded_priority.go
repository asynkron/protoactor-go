package mailbox

import (
	"github.com/asynkron/protoactor-go/internal/queue/goring"
	"github.com/asynkron/protoactor-go/internal/queue/mpsc"
)

func NewPriorityGoringQueue() *priorityQueue {
	return NewPriorityQueue(func() queue {
		return &unboundedMailboxQueue{
			userMailbox: goring.New(10),
		}
	})
}

func UnboundedPriority(mailboxStats ...Middleware) Producer {
	return func() Mailbox {
		return &defaultMailbox{
			systemMailbox: mpsc.New(),
			userMailbox:   NewPriorityGoringQueue(),
			mailboxStats:  mailboxStats,
		}
	}
}

func NewPriorityMpscQueue() *priorityQueue {
	return NewPriorityQueue(func() queue {
		return mpsc.New()
	})
}

func UnboundedPriorityMpsc(mailboxStats ...Middleware) Producer {
	return func() Mailbox {
		return &defaultMailbox{
			systemMailbox: mpsc.New(),
			userMailbox:   NewPriorityMpscQueue(),
			mailboxStats:  mailboxStats,
		}
	}
}
