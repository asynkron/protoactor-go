package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"nats-stream-locality/shared"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/cluster/clusterproviders/consul"
	"github.com/asynkron/protoactor-go/cluster/identitylookup/disthash"
	"github.com/asynkron/protoactor-go/remote"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var subjects = flag.String("subjects", "sensors.>", "Comma-separated NATS subject patterns to subscribe to")

// SensorGrain implements the Sensor virtual actor.
type SensorGrain struct {
	readingCount int32
}

func (s *SensorGrain) Init(ctx cluster.GrainContext) {
	ctx.Logger().Info("SensorGrain activated", slog.String("identity", ctx.Identity()))
}

func (s *SensorGrain) Terminate(ctx cluster.GrainContext) {
	ctx.Logger().Info("SensorGrain deactivated",
		slog.String("identity", ctx.Identity()),
		slog.Int("totalReadings", int(s.readingCount)),
	)
}

func (s *SensorGrain) ReceiveDefault(ctx cluster.GrainContext) {}

func (s *SensorGrain) HandleReading(req *shared.SensorReading, ctx cluster.GrainContext) (*shared.Ack, error) {
	s.readingCount++
	ctx.Logger().Info("HandleReading",
		slog.String("identity", ctx.Identity()),
		slog.String("sensorId", req.SensorId),
		slog.String("subject", req.Subject),
		slog.Float64("value", req.Value),
		slog.Int("count", int(s.readingCount)),
	)
	return &shared.Ack{}, nil
}

func main() {
	flag.Parse()

	// Connect to NATS
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = nats.DefaultURL
	}
	nc, err := nats.Connect(natsURL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Close()

	// Get JetStream context
	js, err := jetstream.New(nc)
	if err != nil {
		log.Fatalf("Failed to create JetStream context: %v", err)
	}

	// Create or ensure the SENSORS stream exists
	ctx := context.Background()
	_, err = js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:     "SENSORS",
		Subjects: []string{"sensors.>"},
	})
	if err != nil {
		log.Fatalf("Failed to create stream: %v", err)
	}

	// Set up Proto.Actor cluster
	system := actor.NewActorSystem()

	provider, err := consul.New()
	if err != nil {
		log.Fatalf("Failed to create Consul provider: %v", err)
	}

	lookup := disthash.New()
	remoteConfig := remote.Configure("localhost", 0)

	sensorKind := shared.NewSensorKind(func() shared.Sensor {
		return &SensorGrain{}
	}, 0)

	// Set the custom affinity strategy on the sensor kind
	sensorKind.WithMemberStrategy(func(c *cluster.Cluster) cluster.MemberStrategy {
		return shared.NewLocalAffinityStrategy(c)
	})

	clusterConfig := cluster.Configure("nats-stream-locality-cluster", provider, lookup, remoteConfig,
		cluster.WithKinds(sensorKind),
	)

	c := cluster.NewCluster(system, clusterConfig)
	if err := c.StartMember(); err != nil {
		log.Fatalf("Failed to start cluster member: %v", err)
	}

	// Publish subject bindings via gossip so other nodes know what this node subscribes to
	c.Gossip.SetState(shared.SubjectBindingsKey, wrapperspb.String(*subjects))
	fmt.Printf("Node started with subject bindings: %s\n", *subjects)

	// Create a filtered JetStream consumer for this node's subjects
	subjectList := strings.Split(*subjects, ",")
	for i := range subjectList {
		subjectList[i] = strings.TrimSpace(subjectList[i])
	}

	consumerName := fmt.Sprintf("node-%s", system.ID)
	cons, err := js.CreateOrUpdateConsumer(ctx, "SENSORS", jetstream.ConsumerConfig{
		Name:          consumerName,
		FilterSubjects: subjectList,
		AckPolicy:     jetstream.AckExplicitPolicy,
	})
	if err != nil {
		log.Fatalf("Failed to create consumer: %v", err)
	}

	// Start consuming messages in a goroutine with cancellation
	consumeCtx, consumeCancel := context.WithCancel(ctx)
	go func() {
		for {
			select {
			case <-consumeCtx.Done():
				return
			default:
			}

			msgs, err := cons.Fetch(10, jetstream.FetchMaxWait(2*time.Second))
			if err != nil {
				// Timeout or other fetch errors are normal during idle periods
				continue
			}

			for msg := range msgs.Messages() {
				subject := msg.Subject()
				// Use the NATS subject as the sensor identity
				sensorID := subject

				reading := &shared.SensorReading{
					SensorId:  sensorID,
					Subject:   subject,
					Timestamp: time.Now().UnixNano(),
				}

				// Parse value from message data if present
				if len(msg.Data()) > 0 {
					// Data is a simple float string from the publisher
					var val float64
					fmt.Sscanf(string(msg.Data()), "%f", &val)
					reading.Value = val
				}

				// Route to the sensor grain via the cluster
				client := shared.GetSensorGrainClient(c, sensorID)
				_, err := client.HandleReading(reading)
				if err != nil {
					log.Printf("Failed to handle reading for %s: %v", sensorID, err)
				}

				msg.Ack()
			}

			if err := msgs.Error(); err != nil {
				log.Printf("Fetch error: %v", err)
			}
		}
	}()

	fmt.Println("Node consuming messages. Press Ctrl+C to stop.")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("Shutting down...")
	consumeCancel()
	c.Shutdown(true)
}
