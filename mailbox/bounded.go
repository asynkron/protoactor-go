package mailbox

import (
	"github.com/AsynkronIT/protoactor-go/internal/queue/mpsc"
	rbqueue "github.com/Workiva/go-datastructures/queue"
)

type boundedMailboxQueue struct {
	userMailbox *rbqueue.RingBuffer
	dropping    bool
}

func (q *boundedMailboxQueue) Push(m interface{}) {
	if q.dropping {
		if q.userMailbox.Len() > 0 && q.userMailbox.Cap()-1 == q.userMailbox.Len() {
			q.userMailbox.Get()
		}
	}
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
	return bounded(size, false, mailboxStats...)
}

// Bounded dropping returns a producer which creates an bounded mailbox of the specified size that drops front element on push
func BoundedDropping(size int, mailboxStats ...Statistics) Producer {
	return bounded(size, true, mailboxStats...)
}

func bounded(size int, dropping bool, mailboxStats ...Statistics) Producer {
	return func() Mailbox {
		q := &boundedMailboxQueue{
			userMailbox: rbqueue.NewRingBuffer(uint64(size)),
			dropping:    dropping,
		}
		return &defaultMailbox{
			systemMailbox: mpsc.New(),
			userMailbox:   q,
			mailboxStats:  mailboxStats,
		}
	}
}
