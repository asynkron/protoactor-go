package main

import (
        "context"
        "fmt"
        "math/rand"
        "time"

        console "github.com/asynkron/goconsole"
        "github.com/asynkron/protoactor-go/actor"
        "github.com/asynkron/protoactor-go/actor/middleware/opentelemetry"
        "go.opentelemetry.io/otel"
        "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
        "go.opentelemetry.io/otel/propagation"
        "go.opentelemetry.io/otel/sdk/resource"
        sdktrace "go.opentelemetry.io/otel/sdk/trace"
        semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

func main() {
        shutdown := initProvider()
        defer shutdown()

        system := actor.NewActorSystem()
        rootContext := actor.
                NewRootContext(system, nil).
                WithSpawnMiddleware(opentelemetry.TracingMiddleware())

        pid := rootContext.SpawnPrefix(createProps(5), "root")
        for i := 0; i < 3; i++ {
                rootContext.RequestFuture(pid, &request{i}, 10*time.Second).Wait()
        }
        _, _ = console.ReadLine()
}

// initProvider configures an OTLP exporter pointing to the standard OTLP gRPC port (4317)
// and sets up a trace provider used by the middleware.
func initProvider() func() {
        ctx := context.Background()

        exporter, err := otlptracegrpc.New(ctx,
                otlptracegrpc.WithEndpoint("localhost:4317"),
                otlptracegrpc.WithInsecure(),
        )
        if err != nil {
                panic(fmt.Sprintf("failed to create exporter: %v", err))
        }

        res, err := resource.Merge(resource.Default(),
                resource.NewWithAttributes(semconv.SchemaURL,
                        semconv.ServiceName("actor-opentelemetry"),
                ))
        if err != nil {
                panic(fmt.Sprintf("failed to create resource: %v", err))
        }

        tp := sdktrace.NewTracerProvider(
                sdktrace.WithBatcher(exporter),
                sdktrace.WithResource(res),
        )
        otel.SetTracerProvider(tp)
        otel.SetTextMapPropagator(propagation.TraceContext{})

        return func() {
                _ = tp.Shutdown(ctx)
        }
}

func createProps(levels int) *actor.Props {
        if levels <= 1 {
                sleep := time.Duration(rand.Intn(5000))

                return actor.PropsFromFunc(func(c actor.Context) {
                        switch msg := c.Message().(type) {
                        case *request:
                                time.Sleep(sleep * time.Millisecond)
                                if c.Sender() != nil {
                                        c.Respond(&response{i: msg.i})
                                }
                        }
                })
        }

        var childs []*actor.PID
        return actor.PropsFromFunc(func(c actor.Context) {
                switch c.Message().(type) {
                case *actor.Started:
                        for i := 0; i < 3; i++ {
                                childs = append(childs, c.Spawn(createProps(levels-1)))
                        }
                case *request:
                        c.Forward(childs[rand.Intn(len(childs))])
                }
        })
}

type request struct {
        i int
}

type response struct {
        i int
}

