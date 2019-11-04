package main

import (
	"fmt"
	"time"

	console "github.com/AsynkronIT/goconsole"
	"github.com/otherview/protoactor-go/actor"
)

type Hello struct{ Who string }

func Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case Hello:
		context.Respond("Hello " + msg.Who)
	}
}

func main() {
	rootContext := actor.EmptyRootContext
	props := actor.PropsFromFunc(Receive)
	pid := rootContext.Spawn(props)
	result, _ := rootContext.RequestFuture(pid, Hello{Who: "Roger"}, 30*time.Second).Result() // await result

	fmt.Println(result)
	console.ReadLine()
}
