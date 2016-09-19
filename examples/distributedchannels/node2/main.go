package main

import (
	"log"
	"runtime"

	"github.com/AsynkronIT/gam/actor"
	"github.com/AsynkronIT/gam/examples/distributedchannels/messages"
	"github.com/AsynkronIT/gam/remoting"
	"github.com/AsynkronIT/goconsole"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	remoting.Start("127.0.0.1:8080")
	//create the channel
	channel := make(chan *messages.MyMessage)

	//create an actor receiving messages and pushing them onto the channel
	props := actor.FromFunc(func(context actor.Context) {
		switch msg := context.Message().(type) {
		case *messages.MyMessage:
			channel <- msg
		}
	})
	actor.SpawnNamed(props, "MyMessage")

	//consume the channel just like you use to
	go func() {
		for msg := range channel {
			log.Println(msg)
		}
	}()

	console.ReadLine()
}
