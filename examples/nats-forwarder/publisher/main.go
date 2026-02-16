package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"nats-forwarder/shared"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

func main() {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = nats.DefaultURL
	}

	workerName := "worker-1"
	if len(os.Args) > 1 {
		workerName = os.Args[1]
	}

	// Connect to NATS
	nc, err := nats.Connect(natsURL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS at %s: %v", natsURL, err)
	}
	defer nc.Close()
	slog.Info("Connected to NATS", slog.String("url", natsURL))

	actions := []string{"process", "validate", "transform", "aggregate"}

	for i := range 20 {
		action := actions[i%len(actions)]
		subject := fmt.Sprintf("commands.%s.%s", workerName, action)
		payload := []byte(fmt.Sprintf(`{"iteration":%d,"worker":"%s","action":"%s"}`, i, workerName, action))

		slog.Info("Sending request",
			slog.String("subject", subject),
			slog.Int("iteration", i),
			slog.String("action", action),
		)

		// Use NATS request-reply pattern
		resp, err := nc.Request(subject, payload, 5*time.Second)
		if err != nil {
			slog.Error("Request failed",
				slog.String("subject", subject),
				slog.Any("error", err),
			)
			continue
		}

		// Unmarshal the response
		result := &shared.CommandResult{}
		if err := proto.Unmarshal(resp.Data, result); err != nil {
			slog.Error("Failed to unmarshal response",
				slog.Any("error", err),
			)
			continue
		}

		slog.Info("Received response",
			slog.String("worker", result.Worker),
			slog.String("action", result.Action),
			slog.Bool("success", result.Success),
			slog.String("result", result.Result),
		)

		time.Sleep(500 * time.Millisecond)
	}

	fmt.Println("Publisher finished sending 20 commands.")
}
