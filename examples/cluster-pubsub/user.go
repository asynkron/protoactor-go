package main

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/scheduler"
)

const Topic = "chat"

var messages = []string{
	"Good day sir!",
	"Lovely weather, innit?",
	"How do you do?",
	"Pardon me!",
}

func randomMessage() string {
	return messages[rand.Intn(len(messages))]
}

type Tick struct{}

type User struct {
	cancelTick scheduler.CancelFunc
}

func (u *User) Init(ctx cluster.GrainContext) {
}

func (u *User) Terminate(ctx cluster.GrainContext) {
	if u.cancelTick != nil {
		u.cancelTick()
	}
}

func (u *User) ReceiveDefault(ctx cluster.GrainContext) {
	switch msg := ctx.Message().(type) {
	case *Tick:
		chatMsg := randomMessage()
		fmt.Printf("[SEND] User %s says: %s\n", ctx.Identity(), chatMsg)
		_, err := ctx.Cluster().Publisher().Publish(context.Background(), Topic, &ChatMessage{Message: chatMsg, Sender: ctx.Identity()})
		if err != nil {
			fmt.Println(err)
		}
	case *ChatMessage:
		fmt.Printf("[RECEIVED] User %s received %s from %s\n", ctx.Identity(), msg.Message, msg.Sender)
	}
}

func (u *User) Connect(_ *Empty, context cluster.GrainContext) (*Empty, error) {
	s := scheduler.NewTimerScheduler(context)
	interval := time.Second * time.Duration(rand.Intn(4)+2)
	u.cancelTick = s.SendRepeatedly(interval, interval, context.Self(), &Tick{})

	_, err := context.Cluster().SubscribeByPid(Topic, context.Self())
	return &Empty{}, err
}
