package actor

import "time"

func NewOneForOneStrategy(maxNrOfRetries int, withinDuration time.Duration, decider Decider) SupervisorStrategy {
	return &OneForOneStrategy{
		maxNrOfRetries: maxNrOfRetries,
		withinDuration: withinDuration,
		decider:        decider,
	}
}

type OneForOneStrategy struct {
	maxNrOfRetries int
	withinDuration time.Duration
	decider        Decider
}

func (strategy *OneForOneStrategy) HandleFailure(supervisor Supervisor, child *PID, rs *RestartStatistics, reason interface{}, message interface{}) {
	directive := strategy.decider(child, reason)

	switch directive {
	case ResumeDirective:
		//resume the failing child
		logFailure(child, reason, directive)
		child.sendSystemMessage(resumeMailboxMessage)
	case RestartDirective:
		//try restart the failing child
		if strategy.requestRestartPermission(rs) {
			logFailure(child, reason, RestartDirective)
			child.sendSystemMessage(restartMessage)
		} else {
			logFailure(child, reason, StopDirective)
			child.Stop()
		}
	case StopDirective:
		//stop the failing child, no need to involve the crs
		logFailure(child, reason, directive)
		child.Stop()
	case EscalateDirective:
		//send failure to parent
		//supervisor mailbox
		//do not log here, log in the parent handling the error
		supervisor.EscalateFailure(reason, message)
	}
}

func (strategy *OneForOneStrategy) requestRestartPermission(rs *RestartStatistics) bool {

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
