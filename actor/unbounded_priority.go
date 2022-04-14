package actor

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

//goland:noinspection ALL
func UnboundedPriority(mailboxStats ...MailboxMiddleware) MailboxProducer {
	return func() Mailbox {
		return &defaultMailbox{
			systemMailbox: mpsc.New(),
			userMailbox:   NewPriorityGoringQueue(),
			middlewares:   mailboxStats,
		}
	}
}

func NewPriorityMpscQueue() *priorityQueue {
	return NewPriorityQueue(func() queue {
		return mpsc.New()
	})
}

func UnboundedPriorityMpsc(mailboxStats ...MailboxMiddleware) MailboxProducer {
	return func() Mailbox {
		return &defaultMailbox{
			systemMailbox: mpsc.New(),
			userMailbox:   NewPriorityMpscQueue(),
			middlewares:   mailboxStats,
		}
	}
}
