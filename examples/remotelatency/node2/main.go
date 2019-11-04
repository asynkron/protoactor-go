package main

import (
	"log"
	"runtime"
	"sort"
	"time"

	"fmt"

	console "github.com/AsynkronIT/goconsole"
	"github.com/otherview/protoactor-go/actor"
	"github.com/otherview/protoactor-go/examples/remotelatency/messages"
	"github.com/otherview/protoactor-go/mailbox"
	"github.com/otherview/protoactor-go/remote"
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

			data := make(map[string]int64)
			for i := 0; i < 100; i++ {
				key := fmt.Sprintf("%v", i)
				data[key] = state.messages[i*10000]
			}

			data["99.9"] = state.messages[999000]
			data["99.99"] = state.messages[999900]

			log.Println("Done")
			for k, v := range data {
				log.Printf("%v %v", k, v)
			}
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
	rootContext := actor.EmptyRootContext
	props := actor.
		PropsFromProducer(newRemoteActor()).
		WithMailbox(mailbox.Bounded(1000))

	rootContext.SpawnNamed(props, "remote")

	console.ReadLine()
}
