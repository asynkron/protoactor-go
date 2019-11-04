package main

import (
	"log"

	console "github.com/AsynkronIT/goconsole"
	"github.com/otherview/protoactor-go/actor"
	"github.com/otherview/protoactor-go/mailbox"
)

type mailboxLogger struct{}

func (m *mailboxLogger) MailboxStarted() {
	log.Print("Mailbox started")
}
func (m *mailboxLogger) MessagePosted(msg interface{}) {
	log.Printf("Message posted %v", msg)
}
func (m *mailboxLogger) MessageReceived(msg interface{}) {
	log.Printf("Message received %v", msg)
}
func (m *mailboxLogger) MailboxEmpty() {
	log.Print("No more messages")
}

func main() {
	rootContext := actor.EmptyRootContext
	props := actor.PropsFromFunc(func(ctx actor.Context) {

	}).WithMailbox(mailbox.Unbounded(&mailboxLogger{}))
	pid := rootContext.Spawn(props)
	rootContext.Send(pid, "Hello")
	console.ReadLine()
}
