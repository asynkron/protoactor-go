package mailbox

import (
	"github.com/AsynkronIT/protoactor-go/internal/queue/mpsc"
)

// NewUnboundedLockfreeMailbox creates an unbounded, lock-free mailbox
func NewUnboundedLockfreeProducer(mailboxStats ...Statistics) Producer {
	return func(invoker MessageInvoker, dispatcher Dispatcher) Inbound {
		return &DefaultMailbox{
			userMailbox:   mpsc.New(),
			systemMailbox: mpsc.New(),
			invoker:       invoker,
			mailboxStats:  mailboxStats,
			dispatcher:    dispatcher,
		}
	}
}
