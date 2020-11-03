package eventstream_test

import (
	"fmt"

	"github.com/AsynkronIT/protoactor-go/eventstream"
)

// Subscribe subscribes to events
func ExampleEventStream_Subscribe() {
	es := eventstream.NewEventStream()
	sub := es.Subscribe(func(event interface{}) {
		fmt.Println(event)
	})

	// only allow strings
	sub.WithPredicate(func(evt interface{}) bool {
		_, ok := evt.(string)
		return ok
	})

	es.Publish("Hello World")
	es.Publish(1)

	es.Unsubscribe(sub)

	// Output: Hello World
}
