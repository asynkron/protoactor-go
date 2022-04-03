package main

import (
	"context"
	"flag"
	"log"
	"sync/atomic"
	"time"

	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"golang.org/x/time/rate"
)

type hello struct {
	Who string
}

func main() {
	irate := flag.Int("rate", 1000000, "How many messages per second")
	throttle := flag.Int("throttle", 5, "Throttle of deadletter logs")
	d := flag.Duration("duration", 10*time.Second, "How long you want to keep sending")
	flag.Parse()

	// init
	cfg := actor.Configure(actor.WithDeadLetterThrottleCount(int32(*throttle)))
	system := actor.NewActorSystemWithConfig(cfg)

	btn := int32(1)
	go func() {
		time.Sleep(*d)
		atomic.StoreInt32(&btn, 0)
	}()

	ctx := context.TODO()
	invalidPid := system.NewLocalPID("unknown")
	limiter := rate.NewLimiter(rate.Limit(*irate), *irate)

	log.Printf("started")
	for atomic.LoadInt32(&btn) == 1 {
		system.Root.Send(invalidPid, &hello{Who: "deadleater"})
		// time.Sleep(sleepDrt)
		limiter.Wait(ctx)
	}
	log.Printf("done")
	console.ReadLine()
}
