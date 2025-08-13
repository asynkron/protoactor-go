package main

import (
	"context"
	"fmt"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	actorotel "github.com/asynkron/protoactor-go/actor/middleware/opentelemetry"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/cluster/clusterproviders/test"
	"github.com/asynkron/protoactor-go/cluster/identitylookup/disthash"
	"github.com/asynkron/protoactor-go/remote"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

type node struct {
	c   *cluster.Cluster
	ctx *actor.RootContext
}

func main() {
	shutdown := initProvider()
	defer shutdown()

	agent := test.NewInMemAgent()

	// start server node hosting the Hello kind
	server := startNode(agent, true)
	// start client node without hosting the kind
	client := startNode(agent, false)

	// allow nodes to join the cluster
	time.Sleep(200 * time.Millisecond)

	sendHello(client, "Proto.Actor")

	fmt.Println("Stopping server node to simulate failure")
	server.c.Shutdown(false)

	// this call fails because the server is down
	sendHello(client, "Fails")

	fmt.Println("Restarting server node to recover")
	server = startNode(agent, true)
	time.Sleep(200 * time.Millisecond)

	sendHello(client, "Recovered")

	server.c.Shutdown(true)
	client.c.Shutdown(true)
}

// startNode configures a cluster member and optionally hosts the Hello actor.
func startNode(agent *test.InMemAgent, hostHello bool) node {
	system := actor.NewActorSystem()
	root := actor.NewRootContext(system, nil).WithSpawnMiddleware(actorotel.TracingMiddleware())

	provider := test.NewTestProvider(agent)
	lookup := disthash.New()
	config := remote.Configure("localhost", 0)

	opts := []cluster.ConfigOption{}
	if hostHello {
		props := actor.PropsFromFunc(func(ctx actor.Context) {
			switch msg := ctx.Message().(type) {
			case *HelloRequest:
				ctx.Logger().Info("received hello", "name", msg.Name)
				// each handled message emits a trace span via the tracing middleware
				ctx.Respond(&HelloResponse{Message: "Hello " + msg.Name})
			}
		}).Configure(actor.WithSpawnMiddleware(actorotel.TracingMiddleware()))

		helloKind := cluster.NewKind("hello", props)
		opts = append(opts, cluster.WithKinds(helloKind))
	}

	clusterConfig := cluster.Configure("demo", provider, lookup, config, opts...)
	c := cluster.New(system, clusterConfig)
	c.StartMember()

	return node{c: c, ctx: root}
}

// sendHello sends a request to the Hello actor using tracing spans.
func sendHello(n node, name string) {
	_, span := otel.Tracer("example").Start(context.Background(), "sendHello")
	defer span.End()

	// The tracing middleware picks up the span when the client actor system sends the message.
	// The Hello actor will create a server-side span when processing the request.
	fut, err := n.c.RequestFuture("user", "hello", &HelloRequest{Name: name}, cluster.WithTimeout(time.Second))
	if err != nil {
		fmt.Printf("request error: %v\n", err)
		return
	}
	if _, err := fut.Result(); err != nil {
		fmt.Printf("response error: %v\n", err)
	}
}

// initProvider configures an OpenTelemetry tracer that writes spans to stdout.
func initProvider() func() {
	exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		panic(err)
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("cluster-node-failure"),
		)),
	)
	otel.SetTracerProvider(tp)
	return func() { _ = tp.Shutdown(context.Background()) }
}
