package main

import (
	"flag"
	"os"
	"runtime/pprof"

	"github.com/otherview/protoactor-go/actor"

	"log"
	"sync"

	"runtime"
	"time"

	"github.com/otherview/protoactor-go/mailbox"
)

type Msg struct {
	Sender *actor.PID
}
type Start struct {
	Sender *actor.PID
}

type pingActor struct {
	count        int
	wgStop       *sync.WaitGroup
	messageCount int
	batch        int
	batchSize    int
}

func (state *pingActor) sendBatch(context actor.Context, sender *actor.PID) bool {
	if state.messageCount == 0 {
		return false
	}

	var m interface{} = &Msg{
		Sender: context.Self(),
	}

	for i := 0; i < state.batchSize; i++ {
		context.Send(sender, m)
	}

	state.messageCount -= state.batchSize
	state.batch = state.batchSize
	return true
}

func (state *pingActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case *Start:
		state.sendBatch(context, msg.Sender)

	case *Msg:
		state.batch--
		if state.batch > 0 {
			return
		}

		if !state.sendBatch(context, msg.Sender) {
			state.wgStop.Done()
		}
	}
}

func newPingActor(stop *sync.WaitGroup, messageCount int, batchSize int) actor.Producer {
	return func() actor.Actor {
		return &pingActor{
			wgStop:       stop,
			messageCount: messageCount,
			batchSize:    batchSize,
		}
	}
}

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")
var blockProfile = flag.String("blockprof", "", "execute contention profiling and save results here")

func main() {
	flag.Parse()
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	// Check for lock contention profiling
	if *blockProfile != "" {
		prof, err := os.Create(*blockProfile)
		if err != nil {
			log.Fatal(err)
		}
		runtime.SetBlockProfileRate(1)
		defer func() {
			pprof.Lookup("block").WriteTo(prof, 0)
		}()
	}

	runtime.GOMAXPROCS(runtime.NumCPU())
	runtime.GC()

	var wg sync.WaitGroup

	messageCount := 1000000
	batchSize := 100
	tps := []int{300, 400, 500, 600, 700, 800, 900}
	log.Println("Dispatcher Throughput			Elapsed Time			Messages per sec")
	for _, tp := range tps {

		d := mailbox.NewDefaultDispatcher(tp)

		clientProps := actor.
			PropsFromProducer(newPingActor(&wg, messageCount, batchSize)).
			WithMailbox(mailbox.Bounded(batchSize + 10)).
			WithDispatcher(d)
		rootContext := actor.EmptyRootContext

		echoProps := actor.
			PropsFromFunc(
				func(context actor.Context) {
					switch msg := context.Message().(type) {
					case *Msg:
						context.Send(msg.Sender, msg)
					}
				}).
			WithMailbox(mailbox.Bounded(batchSize + 10)).
			WithDispatcher(d)

		clients := make([]*actor.PID, 0)
		echos := make([]*actor.PID, 0)
		clientCount := runtime.NumCPU() * 2
		for i := 0; i < clientCount; i++ {
			client := rootContext.Spawn(clientProps)
			echo := rootContext.Spawn(echoProps)
			clients = append(clients, client)
			echos = append(echos, echo)
			wg.Add(1)
		}
		start := time.Now()

		for i := 0; i < clientCount; i++ {
			client := clients[i]
			echo := echos[i]

			rootContext.Send(client, &Start{
				Sender: echo,
			})
		}

		wg.Wait()
		elapsed := time.Since(start)
		x := int(float32(messageCount*2*clientCount) / (float32(elapsed) / float32(time.Second)))
		log.Printf("			%v			%s			%v", tp, elapsed, x)
		for i := 0; i < clientCount; i++ {
			client := clients[i]
			rootContext.StopFuture(client).Wait()
			echo := echos[i]
			rootContext.StopFuture(echo).Wait()
		}
		runtime.GC()
		time.Sleep(2 * time.Second)
	}

	// f, err := os.Create("memprofile")
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// pprof.WriteHeapProfile(f)
	// f.Close()
}
