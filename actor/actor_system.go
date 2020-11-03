package actor

import (
	"github.com/AsynkronIT/protoactor-go/eventstream"
)

//goland:noinspection GoNameStartsWithPackageName
type ActorSystem struct {
	ProcessRegistry *ProcessRegistryValue
	Root            *RootContext
	EventStream     *eventstream.EventStream
	Guardians       *guardiansValue
	DeadLetter      *deadLetterProcess
}

func (as *ActorSystem) NewLocalPID(id string) *PID {
	return NewPID(as.ProcessRegistry.Address, id)
}

func NewActorSystem() *ActorSystem {
	system := &ActorSystem{}

	system.ProcessRegistry = NewProcessRegistry(system)
	system.Root = NewRootContext(system, EmptyMessageHeader)
	system.Guardians = NewGuardians(system)
	system.EventStream = eventstream.NewEventStream()
	system.DeadLetter = NewDeadLetter(system)
	SubscribeSupervision(system)

	return system
}
