package actor

import (
	"net"
	"strconv"
	"strings"

	"github.com/asynkron/protoactor-go/eventstream"
	"github.com/asynkron/protoactor-go/extensions"
	"github.com/google/uuid"
	"github.com/lithammer/shortuuid/v4"
)

//goland:noinspection GoNameStartsWithPackageName
type ActorSystem struct {
	Id              string
	ProcessRegistry *ProcessRegistryValue
	Root            *RootContext
	EventStream     *eventstream.EventStream
	Guardians       *guardiansValue
	DeadLetter      *deadLetterProcess
	Extensions      *extensions.Extensions
	Config          *Config
	ID              string
}

func (as *ActorSystem) NewLocalPID(id string) *PID {
	return NewPID(as.ProcessRegistry.Address, id)
}

func (as *ActorSystem) Address() string {
	return as.ProcessRegistry.Address
}

func (as *ActorSystem) GetHostPort() (host string, port int, err error) {
	addr := as.ProcessRegistry.Address
	if h, p, e := net.SplitHostPort(addr); e != nil {
		if addr != localAddress {
			err = e
		}
		host = localAddress
		port = -1
	} else {
		host = h
		port, err = strconv.Atoi(p)
	}
	return
}

func (as *ActorSystem) Shutdown() {

}

func NewActorSystem(options ...ConfigOption) *ActorSystem {
	config := Configure(options...)
	return NewActorSystemWithConfig(config)
}

func NewActorSystemWithConfig(config *Config) *ActorSystem {
	system := &ActorSystem{}
	system.Id = shortuuid.New()
	system.Config = config
	system.ProcessRegistry = NewProcessRegistry(system)
	system.Root = NewRootContext(system, EmptyMessageHeader)
	system.Guardians = NewGuardians(system)
	system.EventStream = eventstream.NewEventStream()
	system.DeadLetter = NewDeadLetter(system)
	system.Extensions = extensions.NewExtensions()
	SubscribeSupervision(system)
	system.Extensions.Register(NewMetrics(config.MetricsProvider))

	system.ProcessRegistry.Add(NewEventStreamProcess(system), "eventstream")

	system.ID = strings.Replace(uuid.New().String(), "-", "", -1)

	return system
}
