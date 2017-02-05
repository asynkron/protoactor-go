package actor

import "github.com/AsynkronIT/protoactor-go/eventstream"

// DeciderFunc is a function which is called by a SupervisorStrategy
type DeciderFunc func(reason interface{}) Directive

type SupervisorStrategy interface {
	HandleFailure(supervisor Supervisor, child *PID, rs *RestartStatistics, reason interface{}, message interface{})
}

type Supervisor interface {
	Children() []*PID
	EscalateFailure(reason interface{}, message interface{})
	RestartChildren(pids ...*PID)
	StopChildren(pids ...*PID)
	ResumeChildren(pids ...*PID)
}

func logFailure(child *PID, reason interface{}, directive Directive) {
	eventstream.Publish(&SupervisorEvent{
		Child:     child,
		Reason:    reason,
		Directive: directive,
	})
}

// DefaultDecider
func DefaultDecider(_ interface{}) Directive {
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

