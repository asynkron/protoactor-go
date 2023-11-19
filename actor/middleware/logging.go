package middleware

import (
	"github.com/asynkron/protoactor-go/actor"
	"log/slog"
)

// Logger is message middleware which logs messages before continuing to the next middleware.
func Logger(next actor.ReceiverFunc) actor.ReceiverFunc {
	fn := func(c actor.ReceiverContext, env *actor.MessageEnvelope) {
		message := env.Message
		c.Logger().Info("Actor got message", slog.Any("self", c.Self()), slog.Any("message", message))
		next(c, env)
	}

	return fn
}
