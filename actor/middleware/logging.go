package middleware

import (
	"log"
	"reflect"

	"github.com/AsynkronIT/protoactor-go/actor"
)

func Logger(next actor.ReceiveFunc) actor.ReceiveFunc {
	fn := func(context actor.Context) {
		message := context.Message()
		log.Printf("%v got %v %+v", context.Self(), reflect.TypeOf(message), message)
		next(context)
	}

	return fn
}
