package actor

import "log"

func ConsoleLogging(context Context) {
	message := context.Message()
	log.Printf("%v got %+v", context.Self(), message)
	context.Handle(message)
}
