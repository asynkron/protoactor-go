package main

import (
	"fmt"
	"io"
	"math/rand"
	"time"

	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/actor/middleware/opentracing"
	"github.com/uber/jaeger-client-go"
	jaegercfg "github.com/uber/jaeger-client-go/config"
	jaegerlog "github.com/uber/jaeger-client-go/log"
	"github.com/uber/jaeger-lib/metrics"
)

func main() {
	jaegerCloser := initJaeger()
	defer jaegerCloser.Close()

	system := actor.NewActorSystem()
	rootContext := actor.
		NewRootContext(system, nil).
		WithSpawnMiddleware(opentracing.TracingMiddleware())

	pid := rootContext.SpawnPrefix(createProps(5), "root")
	for i := 0; i < 3; i++ {
		rootContext.RequestFuture(pid, &request{i}, 10*time.Second).Wait()
	}
	_, _ = console.ReadLine()
}

func initJaeger() io.Closer {
	// Sample configuration for testing. Use constant sampling to sample every trace
	// and enable LogSpan to log every span via configured Logger.
	cfg := jaegercfg.Configuration{
		Sampler: &jaegercfg.SamplerConfig{
			Type:  jaeger.SamplerTypeConst,
			Param: 1,
		},
		Reporter: &jaegercfg.ReporterConfig{
			LogSpans: true,
		},
	}

	// Example logger and metrics factory. Use github.com/uber/jaeger-client-go/log
	// and github.com/uber/jaeger-lib/metrics respectively to bind to real logging and metrics
	// frameworks.
	jLogger := jaegerlog.StdLogger
	jMetricsFactory := metrics.NullFactory

	// Initialize tracer with a logger and a metrics factory
	closer, err := cfg.InitGlobalTracer(
		"jaeger-test",
		jaegercfg.Logger(jLogger),
		jaegercfg.Metrics(jMetricsFactory),
	)
	if err != nil {
		// log.Printf("Could not initialize jaeger tracer: %s", err.Error())
		panic(fmt.Sprintf("Could not initialize jaeger tracer: %s", err.Error()))
	}
	return closer
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
