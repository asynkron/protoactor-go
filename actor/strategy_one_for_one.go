package actor

import "time"

// NewOneForOneStrategy returns a new Supervisor strategy which applies the fault Directive from the decider
// to the failing child process.
//
// This strategy is applicable if it is safe to handle a single child in isolation from its peers or dependents
func NewOneForOneStrategy(maxNrOfRetries int, withinDuration time.Duration, decider DeciderFunc) SupervisorStrategy {
	return &oneForOne{
		maxNrOfRetries: maxNrOfRetries,
		withinDuration: withinDuration,
		decider:        decider,
	}
}

type oneForOne struct {
	maxNrOfRetries int
	withinDuration time.Duration
	decider        DeciderFunc
}

func (strategy *oneForOne) HandleFailure(supervisor Supervisor, child *PID, rs *RestartStatistics, reason interface{}, message interface{}) {
	directive := strategy.decider(reason)

	switch directive {
	case ResumeDirective:
		// resume the failing child
		logFailure(child, reason, directive)
		supervisor.ResumeChildren(child)
	case RestartDirective:
		// try restart the failing child
		if strategy.shouldStop(rs) {
			logFailure(child, reason, StopDirective)
			supervisor.StopChildren(child)
		} else {
			logFailure(child, reason, RestartDirective)
			supervisor.RestartChildren(child)
		}
	case StopDirective:
		// stop the failing child, no need to involve the crs
		logFailure(child, reason, directive)
		supervisor.StopChildren(child)
	case EscalateDirective:
		// send failure to parent
		// supervisor mailbox
		// do not log here, log in the parent handling the error
		supervisor.EscalateFailure(reason, message)
	}
}

func (strategy *oneForOne) shouldStop(rs *RestartStatistics) bool {
	// supervisor says this child may not restart
	if strategy.maxNrOfRetries == 0 {
		return true
	}

	rs.Fail()

	if rs.NumberOfFailures(strategy.withinDuration) > strategy.maxNrOfRetries {
		rs.Reset()
		return true
	}

	return false
}
