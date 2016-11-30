package main

import (
	"fmt"
	"time"

	"github.com/AsynkronIT/gam/actor"
)

type request struct {
	num  int
	size int
	div  int
}

var (
	props = actor.FromProducer(newState)
)

type state struct {
	sum     int
	replies int
	replyTo *actor.PID
}

func newState() actor.Actor {
	return &state{}
}

func (s *state) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *request:
		if msg.size == 1 {
			ctx.Respond(msg.num)
			return
		}

		s.replies = msg.div
		s.replyTo = ctx.Sender()
		for i := 0; i < msg.div; i++ {
			ctx.Spawn(props).
				Request(&request{
					num:  msg.num + i*(msg.size/msg.div),
					size: msg.size / msg.div,
					div:  msg.div,
				}, ctx.Self())
		}
	case int:
		s.sum += msg
		s.replies--
		if s.replies == 0 {
			s.replyTo.Tell(s.sum)
		}
	}
}

func main() {
	start := time.Now()
	pid := actor.Spawn(props)
	res, _ := pid.RequestFuture(&request{
		num:  0,
		size: 1000000,
		div:  10,
	}, 10*time.Second).Result()
	result := res.(int)

	took := time.Since(start)
	fmt.Printf("Result: %d in %d ms.\n", result, took.Nanoseconds()/1e6)
}
