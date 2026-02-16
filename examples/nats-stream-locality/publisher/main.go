package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func main() {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = nats.DefaultURL
	}

	nc, err := nats.Connect(natsURL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Close()

	js, err := jetstream.New(nc)
	if err != nil {
		log.Fatalf("Failed to create JetStream context: %v", err)
	}

	// Ensure the SENSORS stream exists
	ctx := context.Background()
	_, err = js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:     "SENSORS",
		Subjects: []string{"sensors.>"},
	})
	if err != nil {
		log.Fatalf("Failed to create stream: %v", err)
	}

	buildings := []string{"building-a", "building-b"}
	floors := []string{"floor-1", "floor-2", "floor-3"}
	types := []string{"temp", "humidity", "pressure"}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	fmt.Println("Publishing 200 sensor readings...")

	for i := 0; i < 200; i++ {
		building := buildings[rng.Intn(len(buildings))]
		floor := floors[rng.Intn(len(floors))]
		sensorType := types[rng.Intn(len(types))]

		subject := fmt.Sprintf("sensors.%s.%s.%s", building, floor, sensorType)
		value := rng.Float64() * 100

		data := fmt.Sprintf("%.2f", value)
		_, err := js.Publish(ctx, subject, []byte(data))
		if err != nil {
			log.Printf("Failed to publish to %s: %v", subject, err)
			continue
		}

		fmt.Printf("[%d] Published %.2f to %s\n", i+1, value, subject)
		time.Sleep(50 * time.Millisecond)
	}

	fmt.Println("Done publishing.")
}
