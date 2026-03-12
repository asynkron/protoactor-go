package actor

import (
	"log/slog"
	"net"
	"strconv"

	"github.com/asynkron/protoactor-go/eventstream"
	"github.com/asynkron/protoactor-go/extensions"
	"github.com/lithammer/shortuuid/v4"
)

// ActorSystem is the runtime environment that hosts actors and manages their
// execution, supervision, and system-wide services.
//
//goland:noinspection GoNameStartsWithPackageName
type ActorSystem struct {
	ProcessRegistry         *ProcessRegistryValue
	Root                    *RootContext
	EventStream             *eventstream.EventStream
	Guardians               *guardiansValue
	DeadLetter              *deadLetterProcess
	Extensions              *extensions.Extensions
	Config                  *Config
	ID                      string
	stopper                 chan struct{}
	logger                  *slog.Logger
	supervisionSubscription *eventstream.Subscription
}

// Logger returns the logger associated with the actor system.
func (as *ActorSystem) Logger() *slog.Logger {
	return as.logger
}

// NewLocalPID creates a PID for a local actor with the given id.
func (as *ActorSystem) NewLocalPID(id string) *PID {
	return NewPID(as.ProcessRegistry.Address, id)
}

// Address returns the network address of the actor system.
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
	if as.supervisionSubscription != nil {
		as.EventStream.Unsubscribe(as.supervisionSubscription)
	}
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

// NewActorSystemWithError creates a new actor system with optional configuration
// options. It returns an error if the configuration is invalid.
func NewActorSystemWithError(options ...ConfigOption) (*ActorSystem, error) {
	config, err := ConfigureWithError(options...)
	if err != nil {
		return nil, err
	}

	return NewActorSystemWithConfig(config), nil
}

// NewActorSystem creates a new actor system with optional configuration
// options. It panics if the configuration is invalid. Use
// NewActorSystemWithError for a non-panicking variant.
func NewActorSystem(options ...ConfigOption) *ActorSystem {
	system, err := NewActorSystemWithError(options...)
	if err != nil {
		panic(err)
	}

	return system
}

// NewActorSystemWithConfig creates a new actor system using an explicit
// configuration struct.
func NewActorSystemWithConfig(config *Config) *ActorSystem {
	system := &ActorSystem{}
	if config.SystemID != "" {
		system.ID = config.SystemID
	} else {
		system.ID = shortuuid.New()
	}
	system.Config = config
	system.logger = config.LoggerFactory(system)
	system.ProcessRegistry = NewProcessRegistry(system)
	system.Root = NewRootContext(system, EmptyMessageHeader)
	system.Guardians = NewGuardians(system)
	system.EventStream = eventstream.NewEventStream()
	system.DeadLetter = NewDeadLetter(system)
	system.Extensions = extensions.NewExtensions()
	system.supervisionSubscription = SubscribeSupervision(system)
	system.Extensions.Register(NewMetrics(system, config.MetricsProvider))

	system.ProcessRegistry.Add(NewEventStreamProcess(system), "eventstream")
	system.stopper = make(chan struct{})

	system.Logger().Info("actor system started", slog.String("id", system.ID))

	return system
}
