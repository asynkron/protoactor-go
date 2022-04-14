package main

import (
	"fmt"
	"time"

	"github.com/asynkron/protoactor-go/actor"
)

// Auto Response in Proto.Actor is a special kind of message that can create its own response message
// it is received just like any other message by the actor
// but the actor context sees the AutoResponse interface and calls GetAutoReplyMessage() to get the response message
// this is useful if you want to guarantee some form of Ack from an actor. without forcing the developer of the actor to
// use context.Respond manually

// e.g. ClusterPubSub feature uses this to Ack back to the Topic actor to let it know the message has been received

type myAutoResponder struct {
	name string
}

func (m myAutoResponder) GetAutoResponse(context actor.Context) interface{} {
	// return some response-message
	// you have full access to the actor context

	return &myAutoResponse{
		name: m.name + " " + context.Self().Id,
	}
}

type myAutoResponse struct {
	name string
}

func main() {
	system := actor.NewActorSystem()
	props := actor.PropsFromFunc(func(ctx actor.Context) {})
	pid := system.Root.Spawn(props)

	res, _ := system.Root.RequestFuture(pid, &myAutoResponder{name: "hello"}, 10*time.Second).Result()

	fmt.Printf("%v", res)
}
