package main

import (
	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/lmittmann/tint"
	slogzap "github.com/samber/slog-zap/v2"
	"go.uber.org/zap"
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

// enable JSON logging
func jsonLogging(system *actor.ActorSystem) *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, nil)).
		With("lib", "Proto.Actor").
		With("system", system.ID)
}

// enable console logging
func consoleLogging(system *actor.ActorSystem) *slog.Logger {
	return slog.Default().
		With("lib", "Proto.Actor").
		With("system", system.ID)
}

// enable colored console logging
func coloredConsoleLogging(system *actor.ActorSystem) *slog.Logger {
	return slog.New(tint.NewHandler(os.Stdout, &tint.Options{
		Level:      slog.LevelDebug,
		TimeFormat: time.RFC3339,
		AddSource:  true,
	})).With("lib", "Proto.Actor").
		With("system", system.ID)
}

// enable Zap logging
func zapAdapterLogging(system *actor.ActorSystem) *slog.Logger {
	zapLogger, _ := zap.NewProduction()

	logger := slog.New(slogzap.Option{Level: slog.LevelDebug, Logger: zapLogger}.NewZapHandler())
	return logger.
		With("lib", "Proto.Actor").
		With("system", system.ID)
}

func main() {

	system := actor.NewActorSystem(actor.WithLoggerFactory(coloredConsoleLogging))

	props := actor.PropsFromProducer(func() actor.Actor { return &helloActor{} })

	pid := system.Root.Spawn(props)
	system.Root.Send(pid, &hello{Who: "Roger"})
	_, _ = console.ReadLine()
}
