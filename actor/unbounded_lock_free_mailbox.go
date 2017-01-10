package actor

import (
	"github.com/AsynkronIT/protoactor-go/internal/queue/mpsc"
)

// NewUnboundedLockfreeMailbox creates an unbounded, lock-free mailbox
func NewUnboundedLockfreeMailbox(mailboxStats ...MailboxStatistics) MailboxProducer {
	return func() Mailbox {
		return &DefaultMailbox{
			userMailbox:   mpsc.New(),
			systemMailbox: mpsc.New(),
			mailboxStats:  mailboxStats,
		}
	}
}
