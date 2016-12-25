package actor

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
type SupervisionStrategy interface {
	HandleFailure(parentCtx Context, allChildren []*PID, child *PID, cause interface{})
}

type OneForOneStrategy struct {
	maxNrOfRetries              int
	withinTimeRangeMilliseconds int
	decider                     Decider
}

func (strategy *OneForOneStrategy) HandleFailure(parentCtx Context, allChildren []*PID, child *PID, reason interface{}) {
	directive := strategy.decider(child, reason)

	switch directive {
	case ResumeDirective:
		//resume the failing child
		child.sendSystemMessage(&ResumeMailbox{})
	case RestartDirective:
		//restart the failing child
		child.sendSystemMessage(&Restart{})
	case StopDirective:
		//stop the failing child
		child.Stop()
	case EscalateDirective:
		//send failure to parent
		//TODO: this is not enough, we need to fail the parentCtx. suspending the mailbox
		//and then escalate upwards
		parentCtx.Parent().sendSystemMessage(&Failure{Reason: reason, Who: child})
	}
}

func NewOneForOneStrategy(maxNrOfRetries int, withinTimeRangeMilliseconds int, decider Decider) SupervisionStrategy {
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

func DefaultSupervisionStrategy() SupervisionStrategy {
	return defaultSupervisionStrategy
}
