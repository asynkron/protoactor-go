package middleware

import (
	"log"
	"reflect"

	"github.com/otherview/protoactor-go/actor"
)

// Logger is message middleware which logs messages before continuing to the next middleware
func Logger(next actor.ReceiverFunc) actor.ReceiverFunc {
	fn := func(c actor.ReceiverContext, env *actor.MessageEnvelope) {
		message := env.Message
		log.Printf("%v got %v %+v", c.Self(), reflect.TypeOf(message), message)
		next(c, env)
	}

	return fn
}
