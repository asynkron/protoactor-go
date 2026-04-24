// Package stream bridges actors with Go channels.
package stream

import (
	"sync/atomic"

	"github.com/awevoke/protoactor-go/actor"
)

// TypedStream converts actor messages of type T into a channel.
type TypedStream[T any] struct {
	c           chan T
	pid         *actor.PID
	actorSystem *actor.ActorSystem
	closed      atomic.Bool
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
	s.closed.Store(true)
	s.actorSystem.Root.Stop(s.pid)
	close(s.c)
}

// NewTypedStream spawns an actor that forwards messages of type T to a channel.
func NewTypedStream[T any](actorSystem *actor.ActorSystem) *TypedStream[T] {
	c := make(chan T)

	s := &TypedStream[T]{
		c:           c,
		pid:         nil, // will be set after spawn
		actorSystem: actorSystem,
	}

	props := actor.PropsFromFunc(func(ctx actor.Context) {
		switch msg := ctx.Message().(type) {
		case actor.AutoReceiveMessage, actor.SystemMessage:
		// ignore terminate
		case T:
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
