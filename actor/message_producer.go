package actor

import "time"

type MessageProducer interface {
	// Tell sends a messages asynchronously to the PID
	Tell(pid *PID, message interface{})

	// Request sends a messages asynchronously to the PID. The actor may send a response back via respondTo, which is
	// available to the receiving actor via Context.Sender
	Request(pid *PID, message interface{}, respondTo *PID)

	// RequestFuture sends a message to a given PID and returns a Future
	RequestFuture(pid *PID, message interface{}, timeout time.Duration) *Future
}

type rootMessageProducer struct {
}

var (
	EmptyContext MessageProducer = &rootMessageProducer{}
)

// Tell sends a messages asynchronously to the PID
func (*rootMessageProducer) Tell(pid *PID, message interface{}) {
	pid.Tell(message)
}

// Request sends a messages asynchronously to the PID. The actor may send a response back via respondTo, which is
// available to the receiving actor via Context.Sender
func (*rootMessageProducer) Request(pid *PID, message interface{}, respondTo *PID) {
	pid.Request(message, respondTo)
}

// RequestFuture sends a message to a given PID and returns a Future
func (*rootMessageProducer) RequestFuture(pid *PID, message interface{}, timeout time.Duration) *Future {
	return pid.RequestFuture(message, timeout)
}
