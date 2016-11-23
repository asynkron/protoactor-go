package main

import (
	"log"
	"runtime"

	"sync"

	"fmt"
	"time"

	"github.com/AsynkronIT/gam/actor"
	"github.com/AsynkronIT/gam/actor/hashing/hashring"
	"github.com/AsynkronIT/gam/remoting"

	"github.com/AsynkronIT/gam/examples/remoterouting/messages"

	console "github.com/AsynkronIT/goconsole"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	runtime.GC()

	remoting.Start("127.0.0.1:8100")

	props := actor.FromGroupRouter(
		remoting.NewRemoteGroupRouter("remote").
			WithDestinationProducer(func(context actor.Context, router actor.RouterState) {
				log.Println("Setting up routees")
				router.SetRoutees(
					remoting.CreateDestinations(context, "remote", []string{"127.0.0.1:8101", "127.0.0.1:8102"}),
				)
			}).
			WithStrategyProducer(func(config actor.GroupRouterConfig) actor.RouterState {
				//return &actor.RoundRobinState{}
				return actor.NewConsistentRouter(config).WithHasher(hashring.New()).ToRouter()
			}),
	)

	remote := actor.Spawn(props)

	messageCount := 1000000

	var wgStop sync.WaitGroup

	props = actor.
		FromProducer(newLocalActor(&wgStop, messageCount)).
		WithMailbox(actor.NewBoundedMailbox(1000, 10000))

	pid := actor.Spawn(props)

	log.Println("Starting to send")

	t := time.Now()

	for i := 0; i < messageCount; i++ {
		message := &messages.Ping{Sender: pid, User: fmt.Sprintf("User_%d", i)}
		remote.Tell(message)
	}

	wgStop.Wait()

	pid.Stop()

	fmt.Printf("elapsed: %v\n", time.Since(t))

	console.ReadLine()
}
