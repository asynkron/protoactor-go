package main

import (
	"fmt"
	"time"

	"github.com/AsynkronIT/goconsole"
	"github.com/AsynkronIT/protoactor-go/actor"
)

type Hello struct{ Who string }

func Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case Hello:
		context.Respond("Hello " + msg.Who)
	}
}

func main() {
	props := actor.FromFunc(Receive)
	pid := actor.Spawn(props)
	result, _ := pid.RequestFuture(Hello{Who: "Roger"}, 30*time.Second).Result() // await result

	fmt.Println(result)
	console.ReadLine()
}
