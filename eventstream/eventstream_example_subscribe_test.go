package eventstream_test

import (
	"fmt"

	"github.com/asynkron/protoactor-go/eventstream"
)

// Subscribe subscribes to events
func ExampleEventStream_Subscribe() {
	es := eventstream.NewEventStream()
	handler := func(event interface{}) {
		fmt.Println(event)
	}

	// only allow strings
	predicate := func(event interface{}) bool {
		_, ok := event.(string)
		return ok
	}

	sub := es.SubscribeWithPredicate(handler, predicate)

	es.Publish("Hello World")
	es.Publish(1)

	es.Unsubscribe(sub)

	// Output: Hello World
}
