package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime/pprof"
	"time"

	"github.com/AsynkronIT/gam/actor"
	"github.com/AsynkronIT/goconsole"
)

type hello struct{ Who string }
type helloActor struct{}

func (state *helloActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case *hello:
		fmt.Printf("Hello %v\n", msg.Who)
	}
}

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")

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

	props := actor.FromInstance(&helloActor{}).WithMailbox(actor.NewUnboundedMailbox(10))
	s := time.Now()
	for i := 0; i < 1000000; i++ {
		actor.Spawn(props)
	}
	log.Println(time.Since(s))
	pid := actor.Spawn(props)
	pid.Tell(&hello{Who: "Roger"})
	console.ReadLine()
}
