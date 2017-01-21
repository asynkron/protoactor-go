package actor

type Decider func(child *PID, reason interface{}) Directive

type SupervisorStrategy interface {
	HandleFailure(supervisor Supervisor, child *PID, crs *ChildRestartStats, reason interface{}, message interface{})
}

type Supervisor interface {
	Children() []*PID
	EscalateFailure(who *PID, reason interface{}, message interface{})
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

var (
	defaultSupervisionStrategy    = NewOneForOneStrategy(10, 0, DefaultDecider)
	restartingSupervisionStrategy = NewRestartingStrategy()
)

func DefaultSupervisorStrategy() SupervisorStrategy {
	return defaultSupervisionStrategy
}

func RestartingSupervisorStrategy() SupervisorStrategy {
	return restartingSupervisionStrategy
}
