package main

import (
	"log"
	"runtime"
	"strings"

	"github.com/rogeralsing/gam/actor"
	"github.com/rogeralsing/gam/examples/chat/messages"
	"github.com/rogeralsing/gam/remoting"
	"github.com/rogeralsing/goconsole"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	remoting.StartServer("127.0.0.1:0")
	server := actor.NewPID("127.0.0.1:8080", "chatserver")

	//spawn our chat client inline
	props := actor.FromFunc(func(context actor.Context) {
		switch msg := context.Message().(type) {
		case *messages.Connected:
			log.Println(msg.Message)
		case *messages.SayResponse:
			log.Printf("%v: %v", msg.UserName, msg.Message)
		case *messages.NickResponse:
			log.Printf("%v is now known as %v", msg.OldUserName, msg.NewUserName)
		}
	})

	client := actor.Spawn(props)

	server.Tell(&messages.Connect{
		Sender: client,
	})

	nick := "Roger"
	for {

		text, _ := console.ReadLine()
		if strings.HasPrefix(text, "/nick ") {
			newNick := strings.Split(text, " ")[1] //get the first word after /nick
			server.Tell(&messages.NickRequest{
				OldUserName: nick,
				NewUserName: newNick,
			})
			nick = newNick
		} else {
			server.Tell(&messages.SayRequest{
				UserName: nick,
				Message:  text,
			})
		}
	}
}
