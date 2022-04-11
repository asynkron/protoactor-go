package main

import (
	"log"
	"sync/atomic"
	"time"

	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
)

// sent to producer to request more work
type requestMoreWork struct {
	items int
}
type requestWorkBehavior struct {
	tokens   int64
	producer *actor.PID
}

func (m *requestWorkBehavior) MailboxStarted() {
	m.requestMore()
}

func (m *requestWorkBehavior) MessagePosted(msg interface{}) {
}

func (m *requestWorkBehavior) MessageReceived(msg interface{}) {
	atomic.AddInt64(&m.tokens, -1)
	if m.tokens == 0 {
		m.requestMore()
	}
}

func (m *requestWorkBehavior) MailboxEmpty() {
}

func (m *requestWorkBehavior) requestMore() {
	log.Println("Requesting more tokens")
	m.tokens = 50
	system.Root.Send(m.producer, &requestMoreWork{items: 50})
}

type producer struct {
	requestedWork int
	producedWork  int
	worker        *actor.PID
}

func (p *producer) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *actor.Started:
		// spawn our worker
		mb := actor.Unbounded(&requestWorkBehavior{
			producer: ctx.Self(),
		})
		workerProps := actor.PropsFromProducer(func() actor.Actor {
			return &worker{}
		}, actor.WithMailbox(mb))

		p.worker = ctx.Spawn(workerProps)
	case *requestMoreWork:
		p.requestedWork += msg.items
		log.Println("Producer got a new work request")
		ctx.Send(ctx.Self(), &produce{})
	case *produce:
		// produce more work
		log.Println("Producer is producing work")
		p.producedWork++
		ctx.Send(p.worker, &work{p.producedWork})

		// decrease our workload and tell ourselves to produce more work
		if p.requestedWork > 0 {
			p.requestedWork--
			ctx.Send(ctx.Self(), &produce{})
		}
	}
}

type (
	produce struct{}
	worker  struct{}
)

func (w *worker) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *work:
		log.Printf("Worker is working %+v", msg)
		time.Sleep(100 * time.Millisecond)
	}
}

type work struct {
	id int
}

var system = actor.NewActorSystem()

func main() {
	producerProps := actor.PropsFromProducer(func() actor.Actor { return &producer{} })
	system.Root.Spawn(producerProps)

	_, _ = console.ReadLine()
}
