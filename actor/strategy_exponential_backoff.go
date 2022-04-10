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

var _ SupervisorStrategy = &exponentialBackoffStrategy{}

func (strategy *exponentialBackoffStrategy) HandleFailure(actorSystem *ActorSystem, supervisor Supervisor, child *PID, rs *RestartStatistics, reason interface{}, _ interface{}) {
	strategy.setFailureCount(rs)

	backoff := rs.FailureCount() * int(strategy.initialBackoff.Nanoseconds())
	noise := rand.Intn(500)
	dur := time.Duration(backoff + noise)
	time.AfterFunc(dur, func() {
		logFailure(actorSystem, child, reason, RestartDirective)
		supervisor.RestartChildren(child)
	})
}

func (strategy *exponentialBackoffStrategy) setFailureCount(rs *RestartStatistics) {
	if rs.NumberOfFailures(strategy.backoffWindow) == 0 {
		rs.Reset()
	}

	rs.Fail()
}
