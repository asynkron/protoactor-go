package mailbox

import (
	"github.com/AsynkronIT/protoactor-go/internal/queue/mpsc"
)

// UnboundedLockfree returns a producer which creates an unbounded, lock-free mailbox.
// This mailbox is cheaper to allocate, but has a slower throughput than the plain Unbounded mailbox.
func UnboundedLockfree(mailboxStats ...Statistics) Producer {
	return func() Mailbox {
		return &defaultMailbox{
			userMailbox:   mpsc.New(),
			systemMailbox: mpsc.New(),
			mailboxStats:  mailboxStats,
		}
	}
}
