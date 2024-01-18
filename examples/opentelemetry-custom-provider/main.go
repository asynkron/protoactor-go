package main

import (
	"context"
	"fmt"
	"log"
	"time"

	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
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

	// Set up resource.
	res, err := newResource("test-service", "0.1.0")
	if err != nil {
		log.Fatalf("failed to create resource: %v", err)
	}

	meterProvider, err := newMeterProvider(res)
	if err != nil {
		log.Fatalf("failed to create meter provider: %v", err)
	}
	defer func() {
		err := meterProvider.Shutdown(ctx)
		if err != nil {
			log.Fatalf("failed to shutdown meter provider: %v", err)
		}
	}()

	config := actor.Configure(actor.WithMetricProviders(meterProvider))
	system := actor.NewActorSystemWithConfig(config)
	props := actor.PropsFromProducer(func() actor.Actor {
		return &helloActor{}
	})

	pid := system.Root.Spawn(props)
	system.Root.Request(pid, &hello{Who: "Stdout Exporter"})
	time.Sleep(100 * time.Millisecond)
	_, _ = console.ReadLine()
}

func newResource(serviceName, serviceVersion string) (*resource.Resource, error) {
	return resource.Merge(resource.Default(),
		resource.NewWithAttributes(semconv.SchemaURL,
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
		))
}

func newMeterProvider(res *resource.Resource) (*metric.MeterProvider, error) {
	metricExporter, err := stdoutmetric.New()
	if err != nil {
		return nil, err
	}

	meterProvider := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(metric.NewPeriodicReader(metricExporter,
			// Default is 1m. Set to 3s for demonstrative purposes.
			metric.WithInterval(3*time.Second))),
	)
	return meterProvider, nil
}
