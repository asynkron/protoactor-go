package chatserver

import (
	"bufio"
	"os"
	"runtime"

	"github.com/emirpasic/gods/sets/hashset"
	"github.com/rogeralsing/gam/actor"
	"github.com/rogeralsing/gam/examples/chat/messages"
	"github.com/rogeralsing/gam/remoting"
)

type server struct {
	clients *hashset.Set
}

func (state *server) notifyAll(message interface{}) {
	for _, tmp := range state.clients.Values() {
		client := tmp.(*actor.PID)
		client.Tell(message)
	}
}

func (state *server) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case messages.Connect:
		state.clients.Add(msg.Sender)
		msg.Sender.Tell(&messages.Connected{Message: "Welcome!"})
	case messages.SayRequest:
		state.notifyAll(&messages.SayResponse{
			UserName: msg.UserName,
			Message:  msg.Message,
		})
	case messages.NickRequest:
		state.notifyAll(&messages.NickResponse{
			OldUserName: msg.OldUserName,
			NewUserName: msg.NewUserName,
		})
	}
}

func newServer() actor.ActorProducer {
	return func() actor.Actor {
		return &server{
			clients: hashset.New(),
		}
	}
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	remoting.StartServer("127.0.0.1:8080")
	pid := actor.SpawnFunc(newServer())
	actor.ProcessRegistry.Register("chatserver", pid)
	bufio.NewReader(os.Stdin).ReadString('\n')
}
