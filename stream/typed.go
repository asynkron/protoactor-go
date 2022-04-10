package stream

import "github.com/asynkron/protoactor-go/actor"

type TypedStream[T any] struct {
	c           chan T
	pid         *actor.PID
	actorSystem *actor.ActorSystem
}

func (s *TypedStream[T]) C() <-chan T {
	return s.c
}

func (s *TypedStream[T]) PID() *actor.PID {
	return s.pid
}

func (s *TypedStream[T]) Close() {
	s.actorSystem.Root.Stop(s.pid)
	close(s.c)
}

func NewTypedStream[T any](actorSystem *actor.ActorSystem) *TypedStream[T] {
	c := make(chan T)

	props := actor.PropsFromFunc(func(ctx actor.Context) {
		switch msg := ctx.Message().(type) {
		case actor.AutoReceiveMessage, actor.SystemMessage:
		// ignore terminate
		case T:
			c <- msg
		}
	})
	pid := actorSystem.Root.Spawn(props)

	return &TypedStream[T]{
		c:           c,
		pid:         pid,
		actorSystem: actorSystem,
	}
}
