package actor

import (
	"log"
	"reflect"
)

func MessageLogging(context Context) {
	message := context.Message()
	log.Printf("%v got %v %+v", context.Self(), reflect.TypeOf(message), message)
	context.Next()
}

func AutoReceive(context Context) {
	switch context.Message().(type) {
	case *PoisonPill:
		panic("Poison Pill")
	default:
		context.Next()
	}
}
