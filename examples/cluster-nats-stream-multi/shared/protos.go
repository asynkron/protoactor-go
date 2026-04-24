package shared

import "github.com/awevoke/protoactor-go/actor"

const HelloKind = "hello"

// HelloActor is a simple grain that responds to string messages.
type HelloActor struct{}

func (h *HelloActor) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *actor.Started:
		ctx.Logger().Info("HelloActor started")
	case string:
		ctx.Respond("Hello, " + msg + "!")
	}
}
