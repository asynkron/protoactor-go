package main

import "github.com/rogeralsing/gam/actor"
import "github.com/rogeralsing/gam/remoting"
import "fmt"
import "bufio"
import "os"
import "github.com/rogeralsing/gam/examples/remoting/messages"

type MyActor struct{
	count int
}

func (state *MyActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case string:
		fmt.Println(msg)
	case *messages.Response:
		state.count++
		if state.count % 1000 == 0 {
			fmt.Println(state.count)
		}
	}
}

func main() {
	remoting.StartServer("localhost:8090")

	pid := actor.SpawnTemplate(&MyActor{})
	message := &messages.Echo{Message: "hej", Sender: pid}
	remote := actor.NewPID("localhost:8091", "foo")
	for i := 0; i < 100000; i++ {
		remote.Tell(message)
	}

	bufio.NewReader(os.Stdin).ReadString('\n')
}
