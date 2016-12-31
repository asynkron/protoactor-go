package actor

import "github.com/AsynkronIT/gam/languages/golang/src/actor/lfqueue"

// NewUnboundedLockfreeMailbox creates an unbounded, lock-free mailbox
func NewUnboundedLockfreeMailbox(mailboxStats ...MailboxStatistics) MailboxProducer {
	return func() Mailbox {
		return &DefaultMailbox{
			userMailbox:   lfqueue.NewLockfreeQueue(),
			systemMailbox: lfqueue.NewLockfreeQueue(),
			mailboxStats:  mailboxStats,
		}
	}
}
