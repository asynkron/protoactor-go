package actor

func NewRestartingStrategy() SupervisorStrategy {
	return &restartingStrategy{}
}

type restartingStrategy struct{}

func (strategy *restartingStrategy) HandleFailure(supervisor Supervisor, child *PID, rs *RestartStatistics, reason interface{}, message interface{}) {
	// always restart
	supervisor.RestartChildren(child)
}
