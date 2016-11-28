package main

import (
	"log"
	"runtime"

	"github.com/AsynkronIT/gam/cluster"
	"github.com/AsynkronIT/gam/experimental/examples/clusterchat/messages"
	"github.com/AsynkronIT/gam/experimental/streams"
	"github.com/AsynkronIT/goconsole"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	cluster.Start("127.0.0.1:0", "127.0.0.1:7711")

	s := streams.NewUntypedStream()
	go func() {
		for msg := range s.C() {
			log.Printf("%+v", msg)
		}
	}()
	server := messages.GetChatServerGrain("server")
	server.Connect(&messages.ConnectRequest{
		ClientStreamPID: s.PID(),
	})

	nick := "Roger"
	cons := console.NewConsole(func(text string) {
		server.Say(&messages.SayRequest{
			UserName: nick,
			Message:  text,
		})
	})
	//write /nick NAME to change your chat username
	cons.Command("/nick", func(newNick string) {
		server.Nick(&messages.NickRequest{
			OldUserName: nick,
			NewUserName: newNick,
		})
	})
	cons.Run()
}
