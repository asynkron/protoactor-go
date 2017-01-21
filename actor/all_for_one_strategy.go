package actor

import "time"

func NewAllForOneStrategy(maxNrOfRetries int, withinDuration time.Duration, decider Decider) SupervisorStrategy {
	return &AllForOneStrategy{
		maxNrOfRetries: maxNrOfRetries,
		withinDuration: withinDuration,
		decider:        decider,
	}
}

type AllForOneStrategy struct {
	maxNrOfRetries int
	withinDuration time.Duration
	decider        Decider
}

func (strategy *AllForOneStrategy) HandleFailure(supervisor Supervisor, child *PID, rs *RestartStatistics, reason interface{}, message interface{}) {
	directive := strategy.decider(child, reason)
	switch directive {
	case ResumeDirective:
		//resume the failing child
		logFailure(child, reason, directive)
		child.sendSystemMessage(resumeMailboxMessage)
	case RestartDirective:
		children := supervisor.Children()
		//try restart the all the children
		if strategy.requestRestartPermission(rs) {
			logFailure(child, reason, RestartDirective)
			for _, c := range children {
				c.sendSystemMessage(restartMessage)
			}

		} else {
			logFailure(child, reason, StopDirective)
			for _, c := range children {
				c.Stop()
			}
		}
	case StopDirective:
		children := supervisor.Children()
		//stop all the children, no need to involve the crs
		logFailure(child, reason, directive)
		for _, c := range children {
			c.Stop()
		}
	case EscalateDirective:
		//send failure to parent
		//supervisor mailbox
		//do not log here, log in the parent handling the error
		supervisor.EscalateFailure(reason, message)
	}
}

func (strategy *AllForOneStrategy) requestRestartPermission(rs *RestartStatistics) bool {

	//supervisor says this child may not restart
	if strategy.maxNrOfRetries == 0 {
		return false
	}

	rs.FailureCount++

	//supervisor says child may restart, and we don't care about any timewindow
	if strategy.withinDuration == 0 {
		//have we restarted fewer times than supervisor allows?
		return rs.FailureCount <= strategy.maxNrOfRetries
	}

	max := time.Now().Add(-strategy.withinDuration)
	if rs.LastFailureTime.After(max) {
		return rs.FailureCount <= strategy.maxNrOfRetries
	}

	//we are past the time limit, we can safely reset the failure count and restart
	rs.FailureCount = 0
	return true
}
