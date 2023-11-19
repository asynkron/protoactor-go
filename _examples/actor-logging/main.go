package main

import (
	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/lmittmann/tint"
	"log/slog"
	"os"
	"time"
)

type (
	hello      struct{ Who string }
	helloActor struct{}
)

func (state *helloActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case *hello:
		context.Logger().Info("Hello ", slog.String("who", msg.Who))
	}
}

func jsonLogging(system *actor.ActorSystem) *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, nil)).
		With("lib", "Proto.Actor").
		With("system", system.ID)
}

func consoleLogging(system *actor.ActorSystem) *slog.Logger {
	return slog.Default().
		With("lib", "Proto.Actor").
		With("system", system.ID)
}

func coloredConsoleLogging(system *actor.ActorSystem) *slog.Logger {
	return slog.New(tint.NewHandler(os.Stdout, &tint.Options{
		Level:      slog.LevelDebug,
		TimeFormat: time.Kitchen,
	})).With("lib", "Proto.Actor").
		With("system", system.ID)
}

func main() {

	system := actor.NewActorSystem(actor.WithLoggerFactory(jsonLogging))

	props := actor.PropsFromProducer(func() actor.Actor { return &helloActor{} })

	pid := system.Root.Spawn(props)
	system.Root.Send(pid, &hello{Who: "Roger"})
	_, _ = console.ReadLine()
}
