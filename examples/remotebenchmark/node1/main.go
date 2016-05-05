package main

import (
	"github.com/rogeralsing/gam/actor"
	"github.com/rogeralsing/gam/remoting"

	"log"
	"sync"

	"time"
)

// import "runtime/pprof"
import "github.com/rogeralsing/gam/examples/remotebenchmark/messages"
import "runtime"

type localActor struct {
	count        int
	wgStart      *sync.WaitGroup
	wgStop       *sync.WaitGroup
	start        time.Time
	messageCount int
}

func (state *localActor) Receive(context actor.Context) {
	switch context.Message().(type) {
	case actor.Started:
		state.wgStart.Add(1)
		state.wgStop.Add(1)
	case *messages.Pong:
		state.count++
		if state.count%50000 == 0 {
			log.Println(state.count)
		}
		if state.count == state.messageCount {
			state.wgStop.Done()
		}
	case *messages.Start:
		state.wgStart.Done()
	}
}

func newLocalActor(start *sync.WaitGroup, stop *sync.WaitGroup, messageCount int) actor.ActorProducer {
	return func() actor.Actor {
		return &localActor{
			wgStart:      start,
			wgStop:       stop,
			messageCount: messageCount,
		}
	}
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	var wgStop sync.WaitGroup
	var wgStart sync.WaitGroup

	messageCount := 1000000

	remoting.Start("127.0.0.1:0")

	props := actor.
		FromProducer(newLocalActor(&wgStart, &wgStop, messageCount)).
		WithMailbox(actor.NewBoundedMailbox(1000, 10000))

	pid := actor.Spawn(props)

	message := &messages.Ping{Sender: pid}
	remote := actor.NewPID("127.0.0.1:8080", "remote")
	remote.Tell(&messages.StartRemote{Sender: pid})

	wgStart.Wait()
	start := time.Now()
	log.Println("Starting to send")

	for i := 0; i < messageCount; i++ {
		remote.Tell(message)
	}

	wgStop.Wait()
	elapsed := time.Since(start)
	log.Printf("Elapsed %s", elapsed)

	x := int(float32(messageCount*2) / (float32(elapsed) / float32(time.Second)))
	log.Printf("Msg per sec %v", x)
}
