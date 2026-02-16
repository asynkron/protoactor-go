package main

import (
	"log/slog"
	"time"

	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
)

type mailboxLogger struct {
	logger *slog.Logger
}

func (m *mailboxLogger) MailboxStarted() {
	m.logger.Info("Mailbox started")
}

func (m *mailboxLogger) MessagePosted(msg any) {
	m.logger.Info("Message posted", slog.Any("message", msg))
}

func (m *mailboxLogger) MessageReceived(msg any) {
	m.logger.Info("Message received", slog.Any("message", msg))
}

func (m *mailboxLogger) MailboxEmpty() {
	m.logger.Info("No more messages")
}

func main() {
	system := actor.NewActorSystem()
	rootContext := system.Root
	props := actor.PropsFromFunc(
		func(ctx actor.Context) {},
		actor.WithMailbox(actor.Unbounded(&mailboxLogger{logger: system.Logger()})),
	)
	pid := rootContext.Spawn(props)
	rootContext.Send(pid, "Hello")
	time.Sleep(1 * time.Second)
	rootContext.Send(pid, "Hello")
	_, _ = console.ReadLine()
}
