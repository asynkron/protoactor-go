package main

import (
	"github.com/AsynkronIT/gam/actor"
	"github.com/AsynkronIT/gam/remoting"

	"log"
	"sync"

	"time"
)

// import "runtime/pprof"
import "github.com/AsynkronIT/gam/examples/remotebenchmark/messages"
import "runtime"

type localActor struct {
	count        int
	wgStop       *sync.WaitGroup
	messageCount int
}

func (state *localActor) Receive(context actor.Context) {
	switch context.Message().(type) {
	case *messages.Pong:
		state.count++
		if state.count%50000 == 0 {
			log.Println(state.count)
		}
		if state.count == state.messageCount {
			state.wgStop.Done()
		}
	}
}

func newLocalActor(stop *sync.WaitGroup, messageCount int) actor.Producer {
	return func() actor.Actor {
		return &localActor{
			wgStop:       stop,
			messageCount: messageCount,
		}
	}
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU() * 1)

	var wg sync.WaitGroup

	messageCount := 1000000

	remoting.Start("127.0.0.1:8081")

	props := actor.
		FromProducer(newLocalActor(&wg, messageCount)).
		WithMailbox(actor.NewBoundedMailbox(1000, 1000))

	pid := actor.Spawn(props)

	remote := actor.NewPID("127.0.0.1:8080", "remote")
	res, _ := remote.AskFuture(&messages.StartRemote{}, 5*time.Second)
	res.Wait()
	wg.Add(1)

	start := time.Now()
	log.Println("Starting to send")

	message := &messages.Ping{}
	for i := 0; i < messageCount; i++ {
		remote.Ask(message, pid)
	}

	wg.Wait()
	elapsed := time.Since(start)
	log.Printf("Elapsed %s", elapsed)

	x := int(float32(messageCount*2) / (float32(elapsed) / float32(time.Second)))
	log.Printf("Msg per sec %v", x)

	// f, err := os.Create("memprofile")
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// pprof.WriteHeapProfile(f)
	// f.Close()
}
