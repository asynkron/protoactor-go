package main

import (
	"log"
	"math/rand"
	"sync"
	"time"

	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/scheduler"
)

var HelloMessages = []string{
	"Hello",
	"Bonjour",
	"Hola",
	"Zdravstvuyte",
	"Nǐn hǎo",
	"Salve",
	"Konnichiwa",
	"Olá",
}

func main() {
	var wg sync.WaitGroup
	wg.Add(5)

	rand.Seed(time.Now().UnixMicro())
	system := actor.NewActorSystem()

	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	count := 0
	props := actor.PropsFromFunc(func(ctx actor.Context) {
		switch t := ctx.Message().(type) {
		case []string:
			count++
			log.Printf("\t%s, counter value: %d", t[rand.Intn(len(t))], count)
			wg.Done()
		case string:
			log.Printf("\t%s\n", t)
		}
	})

	pid := system.Root.Spawn(props)

	s := scheduler.NewTimerScheduler(system.Root)
	cancel := s.SendRepeatedly(1*time.Millisecond, 1*time.Millisecond, pid, HelloMessages)

	wg.Wait()
	cancel()

	wg.Add(100) // add 100 to our waiting group
	cancel = s.RequestRepeatedly(1*time.Millisecond, 1*time.Millisecond, pid, HelloMessages)

	// the following timer will fire before the
	// wait group is consumed and will stop the scheduler
	time.Sleep(10 * time.Millisecond)
	cancel()

	s.SendOnce(1*time.Millisecond, pid, "Hello Once")

	// this message will never show as we cancel it before it can be fired
	cancel = s.RequestOnce(500*time.Millisecond, pid, "Hello Once Again")
	time.Sleep(250 * time.Millisecond)
	cancel()

	_, _ = console.ReadLine()
}
