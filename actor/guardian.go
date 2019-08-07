package actor

import (
	"errors"
	"sync"

	"github.com/AsynkronIT/protoactor-go/log"
)

type guardiansValue struct {
	guardians *sync.Map
}

var guardians = &guardiansValue{&sync.Map{}}

func (gs *guardiansValue) getGuardianPid(s SupervisorStrategy) *PID {
	if g, ok := gs.guardians.Load(s); ok {
		return g.(*guardianProcess).pid
	}
	g := gs.newGuardian(s)
	gs.guardians.Store(s, g)
	return g.pid
}

// newGuardian creates and returns a new actor.guardianProcess with a timeout of duration d
func (gs *guardiansValue) newGuardian(s SupervisorStrategy) *guardianProcess {
	ref := &guardianProcess{strategy: s}
	id := ProcessRegistry.NextId()

	pid, ok := ProcessRegistry.Add(ref, "guardian"+id)
	if !ok {
		plog.Error("failed to register guardian process", log.Stringer("pid", pid))
	}

	ref.pid = pid
	return ref
}

type guardianProcess struct {
	pid      *PID
	strategy SupervisorStrategy
}

func (g *guardianProcess) SendUserMessage(pid *PID, message interface{}) {
	panic(errors.New("guardian actor cannot receive any user messages"))
}

func (g *guardianProcess) SendSystemMessage(pid *PID, message interface{}) {
	if msg, ok := message.(*Failure); ok {
		g.strategy.HandleFailure(g, msg.Who, msg.RestartStats, msg.Reason, msg.Message)
	}
}

func (g *guardianProcess) Stop(pid *PID) {
	// Ignore
}

func (g *guardianProcess) Children() []*PID {
	panic(errors.New("guardian does not hold its children PIDs"))
}

func (*guardianProcess) EscalateFailure(reason interface{}, message interface{}) {
	panic(errors.New("guardian cannot escalate failure"))
}

func (*guardianProcess) RestartChildren(pids ...*PID) {
	for _, pid := range pids {
		pid.sendSystemMessage(restartMessage)
	}
}

func (*guardianProcess) StopChildren(pids ...*PID) {
	for _, pid := range pids {
		pid.sendSystemMessage(stopMessage)
	}
}

func (*guardianProcess) ResumeChildren(pids ...*PID) {
	for _, pid := range pids {
		pid.sendSystemMessage(resumeMailboxMessage)
	}
}
