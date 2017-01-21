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

func (strategy *AllForOneStrategy) HandleFailure(supervisor Supervisor, child *PID, crs *ChildRestartStats, reason interface{}, message interface{}) {
	directive := strategy.decider(child, reason)
	switch directive {
	case ResumeDirective:
		//resume the failing child
		logFailure(child, reason, directive)
		child.sendSystemMessage(resumeMailboxMessage)
	case RestartDirective:
		children := supervisor.Children()
		//try restart the all the children
		if crs.requestRestartPermission(strategy.maxNrOfRetries, strategy.withinDuration) {
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
