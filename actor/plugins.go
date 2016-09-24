package actor

import (
	"log"
	"reflect"
)

func MessageLogging(context Context) {
	message := context.Message()
	log.Printf("%v got %v %+v", context.Self(), reflect.TypeOf(message), message)
	context.Handle(message)
}
