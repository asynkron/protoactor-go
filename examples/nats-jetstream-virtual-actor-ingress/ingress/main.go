package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"nats-jetstream-virtual-actor-ingress/shared"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/cluster"
	"github.com/awevoke/protoactor-go/cluster/clusterproviders/consul"
	"github.com/awevoke/protoactor-go/cluster/identitylookup/disthash"
	"github.com/awevoke/protoactor-go/remote"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	streamName   = "DEVICES"
	consumerName = "device-ingress"
	batchSize    = 50
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

	// Create JetStream context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	js, err := jetstream.New(nc)
	if err != nil {
		log.Fatalf("Failed to create JetStream context: %v", err)
	}

	// Create or update the DEVICES stream
	_, err = js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:     streamName,
		Subjects: []string{"devices.>"},
	})
	if err != nil {
		log.Fatalf("Failed to create/update stream %q: %v", streamName, err)
	}
	slog.Info("Stream ready", slog.String("stream", streamName))

	// Create or update a durable pull consumer with explicit ack
	consumer, err := js.CreateOrUpdateConsumer(ctx, streamName, jetstream.ConsumerConfig{
		Durable:   consumerName,
		AckPolicy: jetstream.AckExplicitPolicy,
	})
	if err != nil {
		log.Fatalf("Failed to create/update consumer %q: %v", consumerName, err)
	}
	slog.Info("Consumer ready", slog.String("consumer", consumerName))

	// Start a cluster client (not a member -- no grain kinds registered)
	system := actor.NewActorSystem()

	provider, err := consul.New()
	if err != nil {
		log.Fatalf("Failed to create Consul provider: %v", err)
	}

	lookup := disthash.New()
	remoteConfig := remote.Configure("localhost", 0)

	clusterConfig := cluster.Configure("nats-js-ingress-cluster", provider, lookup, remoteConfig)

	c := cluster.NewCluster(system, clusterConfig)
	if err := c.StartClient(); err != nil {
		log.Fatalf("Failed to start cluster client: %v", err)
	}
	slog.Info("Cluster client started")

	// Separate goroutine publishes simulated device messages via JetStream
	go func() {
		deviceCount := 100
		interval := 50 * time.Millisecond
		i := 0
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			deviceID := fmt.Sprintf("device-%03d", rand.Intn(deviceCount))
			subject := fmt.Sprintf("devices.%s", deviceID)
			data := fmt.Sprintf("reading-%d", i)

			if _, err := js.Publish(ctx, subject, []byte(data)); err != nil {
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

	// Consume loop: pull batches from the durable consumer
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			batch, err := consumer.Fetch(batchSize, jetstream.FetchMaxWait(2*time.Second))
			if err != nil {
				slog.Error("Fetch error", slog.Any("error", err))
				time.Sleep(500 * time.Millisecond)
				continue
			}

			// Collect all messages from the batch
			var msgs []jetstream.Msg
			for msg := range batch.Messages() {
				msgs = append(msgs, msg)
			}
			if batch.Error() != nil {
				slog.Warn("Batch completed with error", slog.Any("error", batch.Error()))
			}

			if len(msgs) == 0 {
				continue
			}

			start := time.Now()

			// Process each message concurrently through virtual actors
			var wg sync.WaitGroup
			var mu sync.Mutex
			allOK := true

			for _, msg := range msgs {
				// Extract deviceID from subject: "devices.{deviceID}"
				parts := strings.SplitN(msg.Subject(), ".", 2)
				if len(parts) < 2 || parts[1] == "" {
					slog.Warn("Ignoring message with invalid subject", slog.String("subject", msg.Subject()))
					continue
				}
				deviceID := parts[1]
				data := string(msg.Data())

				wg.Add(1)
				go func(deviceID, data string) {
					defer wg.Done()

					client := shared.GetDeviceGrainClient(c, deviceID)
					_, err := client.HandleMessage(&shared.DeviceMessage{Data: data})
					if err != nil {
						slog.Error("Failed to forward message to grain",
							slog.String("deviceID", deviceID),
							slog.Any("error", err),
						)
						mu.Lock()
						allOK = false
						mu.Unlock()
					}
				}(deviceID, data)
			}

			wg.Wait()

			// If all actor calls succeeded, ack every message in the batch
			if allOK {
				for _, msg := range msgs {
					if err := msg.Ack(); err != nil {
						slog.Error("Failed to ack message",
							slog.String("subject", msg.Subject()),
							slog.Any("error", err),
						)
					}
				}
			} else {
				slog.Warn("Some messages failed; not acking batch — they will be redelivered")
			}

			elapsed := time.Since(start)
			throughput := float64(len(msgs)) / elapsed.Seconds()
			slog.Info("Batch processed",
				slog.Int("messages", len(msgs)),
				slog.Duration("elapsed", elapsed),
				slog.Float64("msgs_per_sec", throughput),
				slog.Bool("allOK", allOK),
			)
		}
	}()

	fmt.Println("Ingress running. Publishing simulated device messages via JetStream. Press Ctrl+C to stop.")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("Shutting down...")
	cancel()
	c.Shutdown(true)
}
