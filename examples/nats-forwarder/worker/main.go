package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"nats-forwarder/shared"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/cluster/clusterproviders/consul"
	"github.com/asynkron/protoactor-go/cluster/identitylookup/disthash"
	"github.com/asynkron/protoactor-go/remote"
)

// WorkerActor handles Command messages and responds with CommandResult.
type WorkerActor struct {
	workerName string
}

func (w *WorkerActor) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *actor.Started:
		slog.Info("WorkerActor started", slog.String("worker", w.workerName))
	case *shared.Command:
		slog.Info("Received command",
			slog.String("worker", w.workerName),
			slog.String("action", msg.Action),
			slog.Int("payloadSize", len(msg.Payload)),
		)

		result := &shared.CommandResult{
			Worker:  w.workerName,
			Action:  msg.Action,
			Success: true,
			Result:  fmt.Sprintf("Worker %s processed action %q with %d bytes of payload", w.workerName, msg.Action, len(msg.Payload)),
		}

		ctx.Respond(result)
	}
}

func main() {
	workerName := "worker-1"
	if len(os.Args) > 1 {
		workerName = os.Args[1]
	}

	system := actor.NewActorSystem()

	provider, err := consul.New()
	if err != nil {
		log.Fatalf("Failed to create Consul provider: %v", err)
	}

	lookup := disthash.New()
	remoteConfig := remote.Configure("localhost", 0)

	// Create a cluster Kind named after this worker.
	// When the forwarder sends a cluster.Request with kind=workerName,
	// this kind will handle the message.
	workerProps := actor.PropsFromProducer(func() actor.Actor {
		return &WorkerActor{workerName: workerName}
	})
	workerKind := cluster.NewKind(workerName, workerProps)

	clusterConfig := cluster.Configure("nats-forwarder-cluster", provider, lookup, remoteConfig,
		cluster.WithKinds(workerKind),
	)

	c := cluster.NewCluster(system, clusterConfig)
	if err := c.StartMember(); err != nil {
		log.Fatalf("Failed to start cluster member: %v", err)
	}

	fmt.Printf("Worker %q started. Hosting worker actor kind. Press Ctrl+C to stop.\n", workerName)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("Shutting down...")
	c.Shutdown(true)
}
