package actor

import (
	"math/rand"
	"time"
)

func NewExponentialBackoffStrategy(backoffWindow time.Duration, initialBackoff time.Duration) SupervisorStrategy {
	return &ExponentialBackoffStrategy{}
}

type ExponentialBackoffStrategy struct {
	backoffWindow  time.Duration
	initialBackoff time.Duration
}

func (strategy *ExponentialBackoffStrategy) HandleFailure(supervisor Supervisor, child *PID, rs *RestartStatistics, reason interface{}, message interface{}) {

	strategy.setFailureCount(rs)
	backoff := rs.FailureCount * int(strategy.initialBackoff.Nanoseconds())
	noise := rand.Intn(500)
	dur := time.Duration(backoff + noise)
	time.AfterFunc(dur, func() {
		child.sendSystemMessage(restartMessage)
	})
}

func (strategy *ExponentialBackoffStrategy) setFailureCount(rs *RestartStatistics) {

	rs.FailureCount++

	//if we are within the backoff window, exit early
	max := time.Now().Add(-strategy.backoffWindow)
	if rs.LastFailureTime.After(max) {
		return
	}

	//we are past the backoff limit, reset the failure counter
	rs.FailureCount = 0
}
