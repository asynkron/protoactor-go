package actor

import (
	"log/slog"

	rbqueue "github.com/Workiva/go-datastructures/queue"
	"github.com/asynkron/protoactor-go/internal/queue/mpsc"
)

type boundedMailboxQueue struct {
	userMailbox *rbqueue.RingBuffer
	dropping    bool
}

func (q *boundedMailboxQueue) Push(m interface{}) {
	if q.dropping {
		if q.userMailbox.Len() > 0 && q.userMailbox.Cap()-1 == q.userMailbox.Len() {
			if _, err := q.userMailbox.Get(); err != nil {
				slog.Default().Error("bounded mailbox failed to drop message", slog.Any("error", err))
			}
		}
	}

	if err := q.userMailbox.Put(m); err != nil {
		slog.Default().Error("bounded mailbox failed to enqueue message", slog.Any("error", err))
	}
}

func (q *boundedMailboxQueue) Pop() interface{} {
	if q.userMailbox.Len() > 0 {
		m, err := q.userMailbox.Get()
		if err != nil {
			slog.Default().Error("bounded mailbox failed to dequeue message", slog.Any("error", err))
			return nil
		}

		return m
	}

	return nil
}

// Bounded returns a producer which creates a bounded mailbox of the specified size.
func Bounded(size int, mailboxStats ...MailboxMiddleware) MailboxProducer {
	return bounded(size, false, mailboxStats...)
}

// BoundedDropping returns a producer which creates a bounded mailbox of the specified size that drops front element on push.
func BoundedDropping(size int, mailboxStats ...MailboxMiddleware) MailboxProducer {
	return bounded(size, true, mailboxStats...)
}

func bounded(size int, dropping bool, mailboxStats ...MailboxMiddleware) MailboxProducer {
	return func() Mailbox {
		q := &boundedMailboxQueue{
			userMailbox: rbqueue.NewRingBuffer(uint64(size)),
			dropping:    dropping,
		}

		return &defaultMailbox{
			systemMailbox: mpsc.New(),
			userMailbox:   q,
			middlewares:   mailboxStats,
		}
	}
}
