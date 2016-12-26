package main

import (
	"log"

	"github.com/AsynkronIT/gam/actor"
	console "github.com/AsynkronIT/goconsole"
)

type mailboxLogger struct{}

func (m *mailboxLogger) MessagePosted(msg interface{}) {
	log.Printf("Message posted %v", msg)
}
func (m *mailboxLogger) MessageReceived(msg interface{}) {
	log.Printf("Message received %v", msg)
}
func (m *mailboxLogger) MailboxEmpty() {
	log.Printf("No more messages")
}

func main() {
	props := actor.FromFunc(func(ctx actor.Context) {

	}).WithMailbox(actor.NewUnboundedMailbox(&mailboxLogger{}))
	actor := actor.Spawn(props)
	actor.Tell("Hello")
	console.ReadLine()
}
