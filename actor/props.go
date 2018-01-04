package actor

import (
	"reflect"

	"github.com/AsynkronIT/protoactor-go/mailbox"
)

type InboundMiddleware func(next ActorFunc) ActorFunc
type OutboundMiddleware func(next SenderFunc) SenderFunc

// Props represents configuration to define how an actor should be created
type Props struct {
	actorProducer       Producer
	mailboxProducer     mailbox.Producer
	guardianStrategy    SupervisorStrategy
	supervisionStrategy SupervisorStrategy
	inboundMiddleware   []InboundMiddleware
	outboundMiddleware  []OutboundMiddleware
	dispatcher          mailbox.Dispatcher
	spawner             SpawnFunc
}

func (props *Props) getDispatcher() mailbox.Dispatcher {
	if props.dispatcher == nil {
		return defaultDispatcher
	}
	return props.dispatcher
}

func (props *Props) getSupervisor() SupervisorStrategy {
	if props.supervisionStrategy == nil {
		return defaultSupervisionStrategy
	}
	return props.supervisionStrategy
}

func (props *Props) produceMailbox(invoker mailbox.MessageInvoker, dispatcher mailbox.Dispatcher) mailbox.Inbound {
	if props.mailboxProducer == nil {
		return defaultMailboxProducer(invoker, dispatcher)
	}
	return props.mailboxProducer(invoker, dispatcher)
}

func (props *Props) spawn(id string, parent *PID) (*PID, error) {
	if props.spawner != nil {
		return props.spawner(id, props, parent)
	}
	return DefaultSpawner(id, props, parent)
}

// Assign one or more middlewares to the props
func (props *Props) WithMiddleware(middleware ...InboundMiddleware) *Props {
	props.inboundMiddleware = append(props.inboundMiddleware, middleware...)
	return props
}

func (props *Props) WithOutboundMiddleware(middleware ...OutboundMiddleware) *Props {
	props.outboundMiddleware = append(props.outboundMiddleware, middleware...)
	return props
}

// WithMailbox assigns the desired mailbox producer to the Props.
func (props *Props) WithMailbox(mailbox mailbox.Producer) *Props {
	props.mailboxProducer = mailbox
	return props
}

// WithGuardian assigns a guardian strategy to the Props.
func (props *Props) WithGuardian(guardian SupervisorStrategy) *Props {
	props.guardianStrategy = guardian
	return props
}

// WithSupervisor assigns a supervision strategy to the Props.
func (props *Props) WithSupervisor(supervisor SupervisorStrategy) *Props {
	props.supervisionStrategy = supervisor
	return props
}

// WithDispatcher changes the dispatcher set in the Props.
func (props *Props) WithDispatcher(dispatcher mailbox.Dispatcher) *Props {
	props.dispatcher = dispatcher
	return props
}

// WithSpawnFunc assigns a custom spawn function to the Props; this is intended
// for internal use only.
func (props *Props) WithSpawnFunc(spawn SpawnFunc) *Props {
	props.spawner = spawn
	return props
}

// WithProducer assigns a actor producer to the Props.
func (props *Props) WithProducer(p Producer) *Props {
	props.actorProducer = p
	return props
}

// WithFunc assigns the function f to the Props as the Receive method for
// actors produced by this Props.
func (props *Props) WithFunc(f ActorFunc) *Props {
	props.actorProducer = makeProducerFromInstance(f)
	return props
}

// WithInstance creates a custom actor producer from the given value and
// assigns it to the Props. See FromInstance for details.
func (props *Props) WithInstance(value Actor) *Props {
	props.actorProducer = makeProducerFromInstance(value)
	return props
}

// WithTemplate creates a custom actor producer with the given template and
// assigns it to the Props. See FromTemplate for details.
func (props *Props) WithTemplate(template Actor) *Props {
	props.actorProducer = makeProducerFromTemplate(template)
	return props
}

// FromProducer creates a Props that runs the given Producer function to
// construct actors.
func FromProducer(actorProducer Producer) *Props {
	return &Props{actorProducer: actorProducer}
}

// FromFunc creates a Props that produces an Actor with the given function as
// its Receive method.
func FromFunc(f ActorFunc) *Props {
	return FromInstance(f)
}

// FromInstance creates a Props that produces an Actor from the given value.
// The value is used as-is every time the actor is incarnated; if the value
// is a pointer, the object that it points to is not re-initialized when the
// actor restarts.
//
// This function is mainly useful for producing actors with immutable or
// intentionally persistent state.
func FromInstance(value Actor) *Props {
	return &Props{actorProducer: makeProducerFromInstance(value)}
}

func makeProducerFromInstance(a Actor) Producer {
	return func() Actor {
		return a
	}
}

// FromTemplate creates a Props that produces an Actor by copying from a template.
// If template is a reference (a pointer to the actor's state, or a map), each
// incarnation of the actor will be re-initialized to the content of the template.
// When template is a value type (i.e. primitives, array, struct), a slice, or a
// channel, the template itself is re-used as the actor.
//
// This function makes it convienent to produce simple actors from Go initializers.
// To construct more complex actors, use FromProducer. See FromInstance.
func FromTemplate(template Actor) *Props {
	return &Props{actorProducer: makeProducerFromTemplate(template)}
}

func makeProducerFromTemplate(template Actor) Producer {
	a := reflect.ValueOf(template)
	t := a.Type()
	// Only a map or a pointer (to any value, including struct and slice)
	// can serve as a template.
	switch t.Kind() {
	case reflect.Ptr:
		t := t.Elem()
		return func() Actor {
			obj := reflect.New(t)
			obj.Elem().Set(a.Elem())
			return obj.Interface().(Actor)
		}
	case reflect.Map:
		return func() Actor {
			obj := reflect.MakeMapWithSize(t, a.Len())
			for _, k := range a.MapKeys() {
				obj.SetMapIndex(k, a.MapIndex(k))
			}
			return obj.Interface().(Actor)
		}
	default:
		return makeProducerFromInstance(template)
	}
}

func FromSpawnFunc(spawn SpawnFunc) *Props {
	return &Props{spawner: spawn}
}
