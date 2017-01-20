package actor

func NewOneForOneStrategy(maxNrOfRetries int, withinTimeRangeMilliseconds int, decider Decider) SupervisorStrategy {
	return &OneForOneStrategy{
		maxNrOfRetries:              maxNrOfRetries,
		withinTimeRangeMilliseconds: withinTimeRangeMilliseconds,
		decider:                     decider,
	}
}

type OneForOneStrategy struct {
	maxNrOfRetries              int
	withinTimeRangeMilliseconds int
	decider                     Decider
}

func (strategy *OneForOneStrategy) HandleFailure(supervisor Supervisor, child *PID, crs *ChildRestartStats, reason interface{}) {
	directive := strategy.decider(child, reason)

	switch directive {
	case ResumeDirective:
		//resume the failing child
		logFailure(child, reason, directive)
		child.sendSystemMessage(resumeMailboxMessage)
	case RestartDirective:
		//try restart the failing child
		if crs.requestRestartPermission(strategy.maxNrOfRetries, strategy.withinTimeRangeMilliseconds) {
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
		supervisor.EscalateFailure(child, reason)
	}
}
