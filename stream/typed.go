// Package stream bridges actors with Go channels.
package stream

import "github.com/asynkron/protoactor-go/actor"

// TypedStream converts actor messages of type T into a channel.
type TypedStream[T any] struct {
	c           chan T
	pid         *actor.PID
	actorSystem *actor.ActorSystem
}

// C returns the underlying receive-only channel for messages of type T.
func (s *TypedStream[T]) C() <-chan T {
	return s.c
}

// PID returns the PID of the backing actor.
func (s *TypedStream[T]) PID() *actor.PID {
	return s.pid
}

// Close stops the backing actor and closes the channel.
func (s *TypedStream[T]) Close() {
	s.actorSystem.Root.Stop(s.pid)
	close(s.c)
}

// NewTypedStream spawns an actor that forwards messages of type T to a channel.
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
