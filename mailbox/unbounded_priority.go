package mailbox

import (
	"github.com/AsynkronIT/protoactor-go/internal/queue/goring"
	"github.com/AsynkronIT/protoactor-go/internal/queue/mpsc"
)

func NewPriorityGoringQueue() *priorityQueue {
	return NewPriorityQueue(func() queue {
		return &unboundedMailboxQueue{
			userMailbox: goring.New(10),
		}
	})
}

func UnboundedPriority(mailboxStats ...Statistics) Producer {
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

func UnboundedPriorityMpsc(mailboxStats ...Statistics) Producer {
	return func() Mailbox {
		return &defaultMailbox{
			systemMailbox: mpsc.New(),
			userMailbox:   NewPriorityMpscQueue(),
			mailboxStats:  mailboxStats,
		}
	}
}
