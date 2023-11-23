package actor

import (
	"errors"
	"log/slog"
	"sync"
)

type guardiansValue struct {
	actorSystem *ActorSystem
	guardians   *sync.Map
}

func NewGuardians(actorSystem *ActorSystem) *guardiansValue {
	return &guardiansValue{
		actorSystem: actorSystem,
		guardians:   &sync.Map{},
	}
}

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
	ref := &guardianProcess{
		strategy:  s,
		guardians: gs,
	}
	id := gs.actorSystem.ProcessRegistry.NextId()

	pid, ok := gs.actorSystem.ProcessRegistry.Add(ref, "guardian"+id)
	if !ok {
		gs.actorSystem.Logger().Error("failed to register guardian process", slog.Any("pid", pid))
	}

	ref.pid = pid
	return ref
}

type guardianProcess struct {
	guardians *guardiansValue
	pid       *PID
	strategy  SupervisorStrategy
}

var _ Process = &guardianProcess{}

func (g *guardianProcess) SendUserMessage(_ *PID, _ interface{}) {
	panic(errors.New("guardian actor cannot receive any user messages"))
}

func (g *guardianProcess) SendSystemMessage(_ *PID, message interface{}) {
	if msg, ok := message.(*Failure); ok {
		g.strategy.HandleFailure(g.guardians.actorSystem, g, msg.Who, msg.RestartStats, msg.Reason, msg.Message)
	}
}

func (g *guardianProcess) Stop(_ *PID) {
	// Ignore
}

func (g *guardianProcess) Children() []*PID {
	panic(errors.New("guardian does not hold its children PIDs"))
}

func (g *guardianProcess) EscalateFailure(_ interface{}, _ interface{}) {
	panic(errors.New("guardian cannot escalate failure"))
}

func (g *guardianProcess) RestartChildren(pids ...*PID) {
	for _, pid := range pids {
		pid.sendSystemMessage(g.guardians.actorSystem, restartMessage)
	}
}

func (g *guardianProcess) StopChildren(pids ...*PID) {
	for _, pid := range pids {
		pid.sendSystemMessage(g.guardians.actorSystem, stopMessage)
	}
}

func (g *guardianProcess) ResumeChildren(pids ...*PID) {
	for _, pid := range pids {
		pid.sendSystemMessage(g.guardians.actorSystem, resumeMailboxMessage)
	}
}
