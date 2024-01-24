package main

import (
	"fmt"
	"strconv"

	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
)

// MessageBatch is a message that is sent to the actor and unpacks its payload in the mailbox
// This allows you to group messages together and send them as a single message
// while processing them as individual messages
// this is used by the Cluster PubSub feature to send a batch of messages and then Ack to the entire batch
// In that specific case, both MessageBatch and AutoRespond are required

type myMessageBatch struct {
	messages []interface{}
}

func (m myMessageBatch) GetMessages() []interface{} {
	return m.messages
}

func main() {
	system := actor.NewActorSystem()
	props := actor.PropsFromFunc(func(ctx actor.Context) {
		if m, ok := ctx.Message().(string); ok {
			fmt.Println(m)
		}
	})
	pid := system.Root.Spawn(props)

	messages := make([]interface{}, 0)

	for i := 0; i < 100; i++ {
		messages = append(messages, "Hello"+strconv.Itoa(i))
	}

	batch := &myMessageBatch{
		messages: messages,
	}
	system.Root.Send(pid, batch)

	console.ReadLine()
}
