package actor

import (
	"math/rand"
	"time"
)

// NewExponentialBackoffStrategy creates a new Supervisor strategy that restarts a faulting child using an exponential
// back off algorithm:
//
//	delay =
func NewExponentialBackoffStrategy(backoffWindow time.Duration, initialBackoff time.Duration) SupervisorStrategy {
	return &exponentialBackoffStrategy{
		backoffWindow:  backoffWindow,
		initialBackoff: initialBackoff,
	}
}

type exponentialBackoffStrategy struct {
	backoffWindow  time.Duration
	initialBackoff time.Duration
}

func (strategy *exponentialBackoffStrategy) HandleFailure(supervisor Supervisor, child *PID, rs *RestartStatistics, reason interface{}, message interface{}) {
	strategy.setFailureCount(rs)

	backoff := rs.FailureCount * int(strategy.initialBackoff.Nanoseconds())
	noise := rand.Intn(500)
	dur := time.Duration(backoff + noise)
	time.AfterFunc(dur, func() {
		child.sendSystemMessage(restartMessage)
	})
}

func (strategy *exponentialBackoffStrategy) setFailureCount(rs *RestartStatistics) {
	rs.FailureCount++

	// if we are within the backoff window, exit early
	if time.Since(rs.LastFailureTime) < strategy.backoffWindow {
		return
	}

	//we are past the backoff limit, reset the failure counter
	rs.FailureCount = 0
}
