package mailbox

import (
	"github.com/AsynkronIT/protoactor-go/internal/queue/mpsc"
	rbqueue "github.com/Workiva/go-datastructures/queue"
)

type boundedMailboxQueue struct {
	userMailbox *rbqueue.RingBuffer
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

// Bounded returns a producer which creates an bounded mailbox of the specified size
func Bounded(size int, mailboxStats ...Statistics) Producer {
	return func(invoker MessageInvoker, dispatcher Dispatcher) Inbound {
		q := &boundedMailboxQueue{
			userMailbox: rbqueue.NewRingBuffer(uint64(size)),
		}
		return &defaultMailbox{
			systemMailbox: mpsc.New(),
			userMailbox:   q,
			invoker:       invoker,
			mailboxStats:  mailboxStats,
			dispatcher:    dispatcher,
		}
	}
}
