package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"nats-jetstream-virtual-actor-ingress/shared"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/cluster"
	"github.com/awevoke/protoactor-go/cluster/clusterproviders/consul"
	"github.com/awevoke/protoactor-go/cluster/identitylookup/disthash"
	"github.com/awevoke/protoactor-go/remote"
)

// DeviceGrain implements the Device virtual actor.
// It tracks the number of messages received and the last data payload.
type DeviceGrain struct {
	messageCount int32
	lastData     string
}

func (d *DeviceGrain) Init(ctx cluster.GrainContext) {
	ctx.Logger().Info("DeviceGrain activated", slog.String("identity", ctx.Identity()))
}

func (d *DeviceGrain) Terminate(ctx cluster.GrainContext) {
	ctx.Logger().Info("DeviceGrain deactivated",
		slog.String("identity", ctx.Identity()),
		slog.Int("totalMessages", int(d.messageCount)),
	)
}

func (d *DeviceGrain) ReceiveDefault(ctx cluster.GrainContext) {}

func (d *DeviceGrain) HandleMessage(req *shared.DeviceMessage, ctx cluster.GrainContext) (*shared.Ack, error) {
	d.messageCount++
	d.lastData = req.Data

	ctx.Logger().Info("HandleMessage",
		slog.String("identity", ctx.Identity()),
		slog.String("data", req.Data),
		slog.Int("count", int(d.messageCount)),
	)

	return &shared.Ack{}, nil
}

func main() {
	system := actor.NewActorSystem()

	provider, err := consul.New()
	if err != nil {
		log.Fatalf("Failed to create Consul provider: %v", err)
	}

	lookup := disthash.New()
	remoteConfig := remote.Configure("localhost", 0)

	deviceKind := shared.NewDeviceKind(func() shared.Device {
		return &DeviceGrain{}
	}, 0)

	clusterConfig := cluster.Configure("nats-js-ingress-cluster", provider, lookup, remoteConfig,
		cluster.WithKinds(deviceKind),
	)

	c := cluster.NewCluster(system, clusterConfig)
	if err := c.StartMember(); err != nil {
		log.Fatalf("Failed to start cluster member: %v", err)
	}

	fmt.Println("Node started. Hosting DeviceActor grains. Press Ctrl+C to stop.")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("Shutting down...")
	c.Shutdown(true)
}
