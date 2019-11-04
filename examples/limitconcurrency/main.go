package main

import (
	"log"

	console "github.com/AsynkronIT/goconsole"
	"github.com/otherview/protoactor-go/actor"
	"github.com/otherview/protoactor-go/router"
)

type workItem struct{ i int }

const maxConcurrency = 5

func doWork(ctx actor.Context) {
	if msg, ok := ctx.Message().(*workItem); ok {
		// this is guaranteed to only execute with a max concurrency level of `maxConcurrency`
		log.Printf("%v got message %d", ctx.Self(), msg.i)
	}
}

func main() {
	rootContext := actor.EmptyRootContext
	pid := rootContext.Spawn(router.NewRoundRobinPool(maxConcurrency).WithFunc(doWork))
	for i := 0; i < 1000; i++ {
		rootContext.Send(pid, &workItem{i})
	}
	console.ReadLine()
}
