package actor

import (
	"log/slog"
	"net"
	"strconv"

	"github.com/asynkron/protoactor-go/eventstream"
	"github.com/asynkron/protoactor-go/extensions"
	"github.com/lithammer/shortuuid/v4"
)

//goland:noinspection GoNameStartsWithPackageName
type ActorSystem struct {
	ProcessRegistry *ProcessRegistryValue
	Root            *RootContext
	EventStream     *eventstream.EventStream
	Guardians       *guardiansValue
	DeadLetter      *deadLetterProcess
	Extensions      *extensions.Extensions
	Config          *Config
	ID              string
	stopper         chan struct{}
	logger          *slog.Logger
}

func (as *ActorSystem) Logger() *slog.Logger {
	return as.logger
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
	close(as.stopper)
}

func (as *ActorSystem) IsStopped() bool {
	select {
	case <-as.stopper:
		return true
	default:
		return false
	}
}

func NewActorSystem(options ...ConfigOption) *ActorSystem {
	config := Configure(options...)

	return NewActorSystemWithConfig(config)
}

func NewActorSystemWithConfig(config *Config) *ActorSystem {
	system := &ActorSystem{}
	system.ID = shortuuid.New()
	system.Config = config
	system.logger = config.LoggerFactory(system)
	system.ProcessRegistry = NewProcessRegistry(system)
	system.Root = NewRootContext(system, EmptyMessageHeader)
	system.Guardians = NewGuardians(system)
	system.EventStream = eventstream.NewEventStream()
	system.DeadLetter = NewDeadLetter(system)
	system.Extensions = extensions.NewExtensions()
	SubscribeSupervision(system)
	system.Extensions.Register(NewMetrics(system, config.MetricsProvider))

	system.ProcessRegistry.Add(NewEventStreamProcess(system), "eventstream")
	system.stopper = make(chan struct{})

	system.Logger().Info("actor system started", slog.String("id", system.ID))

	return system
}
