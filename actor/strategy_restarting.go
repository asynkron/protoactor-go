package actor

func NewRestartingStrategy() SupervisorStrategy {
	return &restartingStrategy{}
}

type restartingStrategy struct{}

var _ SupervisorStrategy = &restartingStrategy{}

func (strategy *restartingStrategy) HandleFailure(actorSystem *ActorSystem, supervisor Supervisor, child *PID, _ *RestartStatistics, reason any, _ any) {
	// always restart
	logFailure(actorSystem, child, reason, RestartDirective)
	supervisor.RestartChildren(child)
}
