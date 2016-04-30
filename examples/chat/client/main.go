package main

import (
	"bufio"
	"log"
	"os"
	"runtime"

	"github.com/rogeralsing/gam/actor"
	"github.com/rogeralsing/gam/examples/chat/messages"
	"github.com/rogeralsing/gam/remoting"
)

type client struct {
	userName string
}

func (state *client) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case messages.Connected:
		log.Println(msg.Message)
	case messages.SayResponse:
		log.Printf("%v: %v", msg.UserName, msg.Message)
	case messages.NickResponse:
		log.Printf("%v is now known as %v", msg.OldUserName, msg.NewUserName)
	}
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	remoting.StartServer("127.0.0.1:0")
	server := actor.NewPID("127.0.0.1:8080", "chatserver")
	pid := actor.SpawnTemplate(&client{
		userName: "Roger",
	})
	server.Tell(&messages.Connect{
		Sender: pid,
	})

	bufio.NewReader(os.Stdin).ReadString('\n')
}
