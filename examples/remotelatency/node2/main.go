package main

import (
	"log"
	"runtime"
	"sort"
	"time"

	"github.com/AsynkronIT/gam/actor"
	"github.com/AsynkronIT/gam/examples/remotelatency/messages"
	"github.com/AsynkronIT/gam/remoting"
	"github.com/AsynkronIT/goconsole"
)

type remoteActor struct {
	i        int
	messages []int64
}

func makeTimestamp() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}

type int64arr []int64

func (a int64arr) Len() int           { return len(a) }
func (a int64arr) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a int64arr) Less(i, j int) bool { return a[i] < a[j] }

func (state *remoteActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case *messages.Start:
		context.Sender().Tell(&messages.Started{})
	case *messages.Ping:
		now := makeTimestamp()
		latency := now - msg.Time

		state.messages[state.i] = latency
		state.i++
		if state.i == 1000000 {
			a := int64arr(state.messages)
			sort.Sort(a)

			r0 := state.messages[0]
			r10 := state.messages[100000]
			r50 := state.messages[500000]
			r999 := state.messages[999000]
			r9999 := state.messages[999900]

			log.Println("Done")
			log.Printf(" 0.00 %v", r0)
			log.Printf("10.00 %v", r10)
			log.Printf("50.00 %v", r50)
			log.Printf("99.90 %v", r999)
			log.Printf("99.99 %v", r9999)
		}
	}
}

func newRemoteActor() actor.Producer {
	return func() actor.Actor {
		return &remoteActor{
			i:        0,
			messages: make([]int64, 1000000),
		}
	}
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	runtime.GC()

	remoting.Start("127.0.0.1:8080", remoting.WithBatchSize(10000))
	props := actor.
		FromProducer(newRemoteActor()).
		WithMailbox(actor.NewBoundedMailbox(1000, 1000))

	actor.SpawnNamed(props, "remote")

	console.ReadLine()
}
