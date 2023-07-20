package main

import (
	"context"
	"fmt"
	"log"
	"time"

	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"go.opentelemetry.io/otel"
	stdout "go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/metric"
	controller "go.opentelemetry.io/otel/sdk/metric/controller/basic"
	processor "go.opentelemetry.io/otel/sdk/metric/processor/basic"
	"go.opentelemetry.io/otel/sdk/metric/selector/simple"
)

type (
	hello      struct{ Who string }
	helloActor struct{}
)

func (state *helloActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case *hello:
		fmt.Printf("Hello %s\n", msg.Who)
	}
}

func main() {
	ctx := context.Background()
	provider := stdoutExporter(ctx)
	defer func() {
		if err := provider.(*controller.Controller).Stop(ctx); err != nil {
			log.Fatalf("could not stop push controller: %v", err)
		}
	}()

	config := actor.Configure(actor.WithMetricProviders(provider))
	system := actor.NewActorSystemWithConfig(config)
	props := actor.PropsFromProducer(func() actor.Actor {
		return &helloActor{}
	})

	pid := system.Root.Spawn(props)
	system.Root.Request(pid, &hello{Who: "Stdout Exporter"})
	time.Sleep(100 * time.Millisecond)
	_, _ = console.ReadLine()
}

func stdoutExporter(ctx context.Context) metric.MeterProvider {
	exporter, _ := stdout.New(stdout.WithPrettyPrint())
	provider := controller.New(
		processor.NewFactory(
			simple.NewWithInexpensiveDistribution(),
			exporter,
		),
		controller.WithExporter(exporter),
	)

	if err := provider.Start(ctx); err != nil {
		log.Fatalf("could not start push controller: %v", err)
	}
	otel.SetMeterProvider(provider)

	return provider
}
