package actor

func NewRestartingStrategy() SupervisorStrategy {
	return &RestartingStrategy{}
}

type RestartingStrategy struct {
}

func (strategy *RestartingStrategy) HandleFailure(supervisor Supervisor, child *PID, rs *RestartStatistics, reason interface{}, message interface{}) {
	//always restart
	child.sendSystemMessage(restartMessage)
}
