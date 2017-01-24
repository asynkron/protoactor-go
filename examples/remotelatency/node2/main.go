package main

import (
	"log"
	"runtime"
	"sort"
	"time"

	"github.com/AsynkronIT/goconsole"
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/examples/remotelatency/messages"
	"github.com/AsynkronIT/protoactor-go/mailbox"
	"github.com/AsynkronIT/protoactor-go/remote"
)

type remoteActor struct {
	i        int
	start    int64
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
		context.Respond(&messages.Started{})
	case *messages.Ping:
		now := makeTimestamp()
		latency := now - msg.Time
		if state.i == 0 {
			log.Println("starting")
			state.start = makeTimestamp()
		}

		state.messages[state.i] = latency
		state.i++
		if state.i == 1000000 {
			done := makeTimestamp()
			delta := done - state.start
			log.Printf("Test took %v ms", delta)

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

	remote.Start("127.0.0.1:8080", remote.WithEndpointWriterBatchSize(10000))
	props := actor.
		FromProducer(newRemoteActor()).
		WithMailbox(mailbox.Bounded(1000))

	actor.SpawnNamed(props, "remote")

	console.ReadLine()
}
