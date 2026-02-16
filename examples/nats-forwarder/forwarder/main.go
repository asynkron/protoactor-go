package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"nats-forwarder/shared"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/cluster/clusterproviders/consul"
	"github.com/asynkron/protoactor-go/cluster/identitylookup/disthash"
	"github.com/asynkron/protoactor-go/remote"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

func main() {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = nats.DefaultURL
	}

	// Connect to NATS
	nc, err := nats.Connect(natsURL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS at %s: %v", natsURL, err)
	}
	defer nc.Close()
	slog.Info("Connected to NATS", slog.String("url", natsURL))

	// Start a cluster client (not a member — no actor kinds hosted here)
	system := actor.NewActorSystem()

	provider, err := consul.New()
	if err != nil {
		log.Fatalf("Failed to create Consul provider: %v", err)
	}

	lookup := disthash.New()
	remoteConfig := remote.Configure("localhost", 0)

	clusterConfig := cluster.Configure("nats-forwarder-cluster", provider, lookup, remoteConfig)

	c := cluster.NewCluster(system, clusterConfig)
	if err := c.StartClient(); err != nil {
		log.Fatalf("Failed to start cluster client: %v", err)
	}
	slog.Info("Cluster client started")

	// Subscribe to commands.> wildcard
	// Subject format: commands.{workerName}.{action}
	_, err = nc.Subscribe("commands.>", func(msg *nats.Msg) {
		// Parse subject: commands.{workerName}.{action}
		parts := strings.SplitN(msg.Subject, ".", 3)
		if len(parts) < 3 || parts[1] == "" || parts[2] == "" {
			slog.Warn("Ignoring message with invalid subject format (expected commands.{worker}.{action})",
				slog.String("subject", msg.Subject),
			)
			return
		}

		workerName := parts[1]
		action := parts[2]

		slog.Info("Forwarding command to actor",
			slog.String("subject", msg.Subject),
			slog.String("worker", workerName),
			slog.String("action", action),
		)

		// Create a Command message
		cmd := &shared.Command{
			Action:       action,
			Payload:      msg.Data,
			ReplySubject: msg.Reply,
		}

		// Forward to the cluster actor via cluster.Request
		// identity = workerName, kind = workerName
		resp, err := c.Request(workerName, workerName, cmd)
		if err != nil {
			slog.Error("Failed to forward command to actor",
				slog.String("worker", workerName),
				slog.String("action", action),
				slog.Any("error", err),
			)
			return
		}

		// Check if we got a CommandResult back
		result, ok := resp.(*shared.CommandResult)
		if !ok {
			slog.Warn("Unexpected response type from actor",
				slog.String("worker", workerName),
				slog.String("type", fmt.Sprintf("%T", resp)),
			)
			return
		}

		// If this is a NATS request-reply, send the result back
		if msg.Reply != "" {
			data, err := proto.Marshal(result)
			if err != nil {
				slog.Error("Failed to marshal CommandResult",
					slog.Any("error", err),
				)
				return
			}
			if err := nc.Publish(msg.Reply, data); err != nil {
				slog.Error("Failed to publish reply",
					slog.String("reply", msg.Reply),
					slog.Any("error", err),
				)
			}
		}

		// Also publish result to results.{workerName}
		resultSubject := fmt.Sprintf("results.%s", workerName)
		data, err := proto.Marshal(result)
		if err != nil {
			slog.Error("Failed to marshal CommandResult for results subject",
				slog.Any("error", err),
			)
			return
		}
		if err := nc.Publish(resultSubject, data); err != nil {
			slog.Error("Failed to publish to results subject",
				slog.String("subject", resultSubject),
				slog.Any("error", err),
			)
		}
	})
	if err != nil {
		log.Fatalf("Failed to subscribe to commands.>: %v", err)
	}

	fmt.Println("Forwarder running. Subscribed to commands.> and forwarding to cluster actors. Press Ctrl+C to stop.")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("Shutting down...")
	nc.Drain()
	c.Shutdown(true)
}
