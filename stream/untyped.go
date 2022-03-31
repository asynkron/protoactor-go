package stream

import "github.com/asynkron/protoactor-go/actor"

type UntypedStream struct {
	c           chan interface{}
	pid         *actor.PID
	actorSystem *actor.ActorSystem
}

func (s *UntypedStream) C() <-chan interface{} {
	return s.c
}

func (s *UntypedStream) PID() *actor.PID {
	return s.pid
}

func (s *UntypedStream) Close() {
	s.actorSystem.Root.Stop(s.pid)
	close(s.c)
}

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
