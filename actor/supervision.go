package actor

import "log"

type Directive int

const (
	ResumeDirective Directive = iota
	RestartDirective
	StopDirective
	EscalateDirective
)

type Decider func(child *PID, cause interface{}) Directive

//TODO: as we dont allow remote children or remote SupervisionStrategy
//Instead of letting the parent keep track of child restart stats.
//this info could actually go into each actor, sending it back to the parent as part of the Failure message
type SupervisorStrategy interface {
	HandleFailure(supervisor Supervisor, child *PID, crs *ChildRestartStats, cause interface{})
}

type OneForOneStrategy struct {
	maxNrOfRetries              int
	withinTimeRangeMilliseconds int
	decider                     Decider
}

type Supervisor interface {
	Children() []*PID
	EscalateFailure(who *PID, reason interface{})
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

//TODO: how should this message look? and should we have a setting for turning this on or off?
func logFailure(child *PID, reason interface{}, directive Directive) {
	var dirname string
	switch directive {
	case ResumeDirective:
		dirname = "Resuming"
	case RestartDirective:
		dirname = "Restarting"
	case StopDirective:
		dirname = "Stopping"
	}
	log.Printf("[ACTOR] %v actor '%v' after failure '%v'", dirname, child, reason)
}

func NewOneForOneStrategy(maxNrOfRetries int, withinTimeRangeMilliseconds int, decider Decider) SupervisorStrategy {
	return &OneForOneStrategy{
		maxNrOfRetries:              maxNrOfRetries,
		withinTimeRangeMilliseconds: withinTimeRangeMilliseconds,
		decider:                     decider,
	}
}

func DefaultDecider(child *PID, reason interface{}) Directive {
	return RestartDirective
}

var defaultSupervisionStrategy = NewOneForOneStrategy(10, 3000, DefaultDecider)

func DefaultSupervisionStrategy() SupervisorStrategy {
	return defaultSupervisionStrategy
}
