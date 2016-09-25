package actor

import (
	"log"
	"reflect"
)

func MessageLogging(context Context) {
	message := context.Message()
	log.Printf("%v got %v %+v", context.Self(), reflect.TypeOf(message), message)
	context.Next(message)
}

func AutoReceive(context Context) {
	switch msg := context.Message().(type) {
	case PoisonPill:
		panic("Poison Pill")
	default:
		context.Next(msg)
	}
}
