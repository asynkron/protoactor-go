package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"nats-virtual-actor-ingress/shared"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/cluster"
	"github.com/awevoke/protoactor-go/cluster/clusterproviders/consul"
	"github.com/awevoke/protoactor-go/cluster/identitylookup/disthash"
	"github.com/awevoke/protoactor-go/remote"
	"github.com/nats-io/nats.go"
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

	// Start a cluster client (not a member -- no grain kinds registered)
	system := actor.NewActorSystem()

	provider, err := consul.New()
	if err != nil {
		log.Fatalf("Failed to create Consul provider: %v", err)
	}

	lookup := disthash.New()
	remoteConfig := remote.Configure("localhost", 0)

	clusterConfig := cluster.Configure("nats-ingress-cluster", provider, lookup, remoteConfig)

	c := cluster.NewCluster(system, clusterConfig)
	if err := c.StartClient(); err != nil {
		log.Fatalf("Failed to start cluster client: %v", err)
	}
	slog.Info("Cluster client started")

	// Subscribe to devices.> wildcard
	// Messages on subjects like "devices.sensor-001" will be routed to the
	// DeviceActor grain with identity "sensor-001".
	sub, err := nc.Subscribe("devices.>", func(msg *nats.Msg) {
		// Extract deviceID from subject: "devices.{deviceID}"
		parts := strings.SplitN(msg.Subject, ".", 2)
		if len(parts) < 2 || parts[1] == "" {
			slog.Warn("Ignoring message with invalid subject", slog.String("subject", msg.Subject))
			return
		}
		deviceID := parts[1]
		data := string(msg.Data)

		slog.Info("Received NATS message",
			slog.String("subject", msg.Subject),
			slog.String("deviceID", deviceID),
			slog.String("data", data),
		)

		client := shared.GetDeviceGrainClient(c, deviceID)
		_, err := client.HandleMessage(&shared.DeviceMessage{Data: data})
		if err != nil {
			slog.Error("Failed to forward message to grain",
				slog.String("deviceID", deviceID),
				slog.Any("error", err),
			)
		}
	})
	if err != nil {
		log.Fatalf("Failed to subscribe to devices.>: %v", err)
	}
	defer func() {
		if err := sub.Unsubscribe(); err != nil {
			slog.Error("Failed to unsubscribe", slog.Any("error", err))
		}
	}()
	slog.Info("Subscribed to NATS subject", slog.String("subject", "devices.>"))

	// Separate goroutine publishes simulated device messages
	go func() {
		deviceCount := 100
		interval := 100 * time.Millisecond
		i := 0
		for {
			deviceID := fmt.Sprintf("device-%03d", i%deviceCount)
			subject := fmt.Sprintf("devices.%s", deviceID)
			data := fmt.Sprintf("reading-%d", i)

			if err := nc.Publish(subject, []byte(data)); err != nil {
				slog.Error("Failed to publish simulated message",
					slog.String("subject", subject),
					slog.Any("error", err),
				)
				return
			}

			i++
			time.Sleep(interval)
		}
	}()

	fmt.Println("Ingress running. Publishing simulated device messages. Press Ctrl+C to stop.")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("Shutting down...")
	c.Shutdown(true)
}
