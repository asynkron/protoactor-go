package actor

type rootSupervisorValue struct {
}

var (
	rootSupervisor = &rootSupervisorValue{}
)

func (*rootSupervisorValue) Children() []*PID {
	return nil
}

func (*rootSupervisorValue) EscalateFailure(reason interface{}, message interface{}) {

}

func (*rootSupervisorValue) RestartChildren(pids ...*PID) {
	for _, pid := range pids {
		pid.sendSystemMessage(restartMessage)
	}
}

func (*rootSupervisorValue) StopChildren(pids ...*PID) {
	for _, pid := range pids {
		pid.sendSystemMessage(stopMessage)
	}
}

func (*rootSupervisorValue) ResumeChildren(pids ...*PID) {
	for _, pid := range pids {
		pid.sendSystemMessage(resumeMailboxMessage)
	}
}

func handleRootFailure(msg *Failure) {
	defaultSupervisionStrategy.HandleFailure(rootSupervisor, msg.Who, msg.RestartStats, msg.Reason, msg.Message)
}
