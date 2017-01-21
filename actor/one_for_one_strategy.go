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
		if rs.requestRestartPermission(strategy.maxNrOfRetries, strategy.withinDuration) {
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
