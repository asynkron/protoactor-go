package actor

import (
	"github.com/asynkron/protoactor-go/internal/queue/mpsc"
)

// UnboundedLockfree returns a producer which creates an unbounded, lock-free mailbox.
// This mailbox is cheaper to allocate, but has a slower throughput than the plain Unbounded mailbox.
func UnboundedLockfree(mailboxStats ...MailboxMiddleware) MailboxProducer {
	return func() Mailbox {
		return &defaultMailbox{
			userMailbox:   mpsc.New(),
			systemMailbox: mpsc.New(),
			middlewares:   mailboxStats,
		}
	}
}
