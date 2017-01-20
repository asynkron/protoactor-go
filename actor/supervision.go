package actor

type Decider func(child *PID, cause interface{}) Directive

type SupervisorStrategy interface {
	HandleFailure(supervisor Supervisor, child *PID, crs *ChildRestartStats, cause interface{})
}

type Supervisor interface {
	Children() []*PID
	EscalateFailure(who *PID, reason interface{})
}

func logFailure(child *PID, reason interface{}, directive Directive) {
	event := &SupervisorEvent{
		Child:     child,
		Reason:    reason,
		Directive: directive,
	}
	EventStream.Publish(event)
}

func DefaultDecider(child *PID, reason interface{}) Directive {
	return RestartDirective
}

var defaultSupervisionStrategy = NewOneForOneStrategy(10, 3000, DefaultDecider)

func DefaultSupervisionStrategy() SupervisorStrategy {
	return defaultSupervisionStrategy
}
