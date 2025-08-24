package stream

import "github.com/asynkron/protoactor-go/actor"

// UntypedStream converts all actor messages into a channel of empty interface.
type UntypedStream struct {
	c           chan interface{}
	pid         *actor.PID
	actorSystem *actor.ActorSystem
}

// C returns the underlying receive-only channel.
func (s *UntypedStream) C() <-chan interface{} {
	return s.c
}

// PID returns the PID of the backing actor.
func (s *UntypedStream) PID() *actor.PID {
	return s.pid
}

// Close stops the backing actor and closes the channel.
func (s *UntypedStream) Close() {
	s.actorSystem.Root.Stop(s.pid)
	close(s.c)
}

// NewUntypedStream spawns an actor that forwards all messages to a channel.
func NewUntypedStream(actorSystem *actor.ActorSystem) *UntypedStream {
	c := make(chan interface{})

	props := actor.PropsFromFunc(func(ctx actor.Context) {
		switch msg := ctx.Message().(type) {
		case actor.AutoReceiveMessage, actor.SystemMessage:
		// ignore terminate
		default:
			c <- msg
		}
	})
	pid := actorSystem.Root.Spawn(props)

	return &UntypedStream{
		c:           c,
		pid:         pid,
		actorSystem: actorSystem,
	}
}
