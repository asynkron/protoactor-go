package stream

import (
	"sync/atomic"

	"github.com/asynkron/protoactor-go/actor"
)

// UntypedStream converts all actor messages into a channel of empty interface.
type UntypedStream struct {
	c           chan any
	pid         *actor.PID
	actorSystem *actor.ActorSystem
	closed      atomic.Bool
}

// C returns the underlying receive-only channel.
func (s *UntypedStream) C() <-chan any {
	return s.c
}

// PID returns the PID of the backing actor.
func (s *UntypedStream) PID() *actor.PID {
	return s.pid
}

// Close stops the backing actor and closes the channel.
func (s *UntypedStream) Close() {
	s.closed.Store(true)
	s.actorSystem.Root.Stop(s.pid)
	close(s.c)
}

// NewUntypedStream spawns an actor that forwards all messages to a channel.
func NewUntypedStream(actorSystem *actor.ActorSystem) *UntypedStream {
	c := make(chan any)

	s := &UntypedStream{
		c:           c,
		pid:         nil, // will be set after spawn
		actorSystem: actorSystem,
	}

	props := actor.PropsFromFunc(func(ctx actor.Context) {
		switch msg := ctx.Message().(type) {
		case actor.AutoReceiveMessage, actor.SystemMessage:
		// ignore terminate
		default:
			if !s.closed.Load() {
				func() {
					defer func() { recover() }() // protect against race between check and send
					c <- msg
				}()
			}
		}
	})
	pid := actorSystem.Root.Spawn(props)
	s.pid = pid

	return s
}
