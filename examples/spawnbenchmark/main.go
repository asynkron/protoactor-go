package main

import (
	"fmt"
	"time"

	"github.com/AsynkronIT/gam/actor"
)

func skynet(c chan int, num int, size int, div int) {
	if size == 1 {
		c <- num
		return
	}

	rc := make(chan int)
	var sum int
	for i := 0; i < div; i++ {
		subNum := num + i*(size/div)
		go skynet(rc, subNum, size/div, div)
	}
	for i := 0; i < div; i++ {
		sum += <-rc
	}
	c <- sum
}

type request struct {
	num  int64
	size int64
	div  int64
}

var (
	props = actor.FromProducer(newState)
)

type state struct {
	sum     int64
	replies int64
	replyTo *actor.PID
}

func newState() actor.Actor {
	return &state{}
}

func (s *state) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case int64:
		s.sum += msg
		s.replies--
		if s.replies == 0 {
			s.replyTo.Tell(s.sum)
		}
	case *request:
		if msg.size == 1 {
			ctx.Respond(msg.num)
			return
		}

		s.replies = msg.div
		s.replyTo = ctx.Sender()
		for i := int64(0); i < msg.div; i++ {
			subNum := msg.num + i*(msg.size/msg.div)
			child := ctx.Spawn(props)
			child.Request(&request{
				num:  subNum,
				size: msg.size / msg.div,
				div:  msg.div,
			}, ctx.Self())
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
	result := res.(int64)

	took := time.Since(start)
	fmt.Printf("Result: %d in %d ms.\n", result, took.Nanoseconds()/1e6)
}
