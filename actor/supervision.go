package actor

import "github.com/AsynkronIT/protoactor-go/eventstream"

type Decider func(child *PID, reason interface{}) Directive

type SupervisorStrategy interface {
	HandleFailure(supervisor Supervisor, child *PID, rs *RestartStatistics, reason interface{}, message interface{})
}

type Supervisor interface {
	Children() []*PID
	EscalateFailure(reason interface{}, message interface{})
}

func logFailure(child *PID, reason interface{}, directive Directive) {
	eventstream.Publish(&SupervisorEvent{
		Child:     child,
		Reason:    reason,
		Directive: directive,
	})
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
