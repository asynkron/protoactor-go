package remote

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"time"

	"github.com/asynkron/protoactor-go/extensions"

	"github.com/asynkron/protoactor-go/actor"
	remotemetrics "github.com/asynkron/protoactor-go/remote/metrics"
	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"
)

var extensionID = extensions.NextExtensionID()

// Remote enables communication between actors across network boundaries.
type Remote struct {
	actorSystem    *actor.ActorSystem
	s              *grpc.Server
	edpReader      *endpointReader
	edpManager     *endpointManager
	config         *Config
	kinds          map[string]*actor.Props
	blocklist      *BlockList
	metrics        *remotemetrics.RemoteMetrics
	metricsEnabled bool
}

// NewRemote creates a new Remote extension for the given actor system.
func NewRemote(actorSystem *actor.ActorSystem, config *Config) *Remote {
	r := &Remote{
		actorSystem: actorSystem,
		config:      config,
		kinds:       make(map[string]*actor.Props),
		blocklist:   NewBlockList(),
	}
	for k, v := range config.Kinds {
		r.kinds[k] = v
	}

	if actorSystem.Config.MetricsEnabled {
		r.metrics = remotemetrics.NewRemoteMetrics(actorSystem.Logger())
		r.metricsEnabled = true
	}

	actorSystem.Extensions.Register(r)

	return r
}

// GetRemote retrieves the Remote extension from the actor system.
//
//goland:noinspection GoUnusedExportedFunction
func GetRemote(actorSystem *actor.ActorSystem) *Remote {
	r := actorSystem.Extensions.Get(extensionID)

	return r.(*Remote)
}

// ExtensionID returns the unique ID of the Remote extension.
func (r *Remote) ExtensionID() extensions.ExtensionID {
	return extensionID
}

// BlockList returns the list of blocked members.
func (r *Remote) BlockList() *BlockList { return r.blocklist }

// Start the remote server.
func (r *Remote) Start() {
	grpclog.SetLoggerV2(grpclog.NewLoggerV2(io.Discard, io.Discard, io.Discard))
	lis, err := net.Listen("tcp", r.config.Address())
	if err != nil {
		panic(fmt.Errorf("failed to listen: %v", err))
	}

	var address string
	if r.config.AdvertisedHost != "" {
		address = r.config.AdvertisedHost
	} else {
		address = lis.Addr().String()
	}

	r.actorSystem.ProcessRegistry.RegisterAddressResolver(r.remoteHandler)
	r.actorSystem.ProcessRegistry.Address = address
	r.Logger().Info("Starting remote with address", slog.String("address", address))

	r.edpManager = newEndpointManager(r)
	r.edpManager.start()

	r.s = grpc.NewServer(r.config.ServerOptions...)
	r.edpReader = newEndpointReader(r)
	RegisterRemotingServer(r.s, r.edpReader)
	r.Logger().Info("Starting Proto.Actor server", slog.String("address", address))
	go func() {
		if err := r.s.Serve(lis); err != nil {
			r.Logger().Error("gRPC server stopped", slog.Any("error", err))
		}
	}()
}

// Shutdown stops the remote server. If graceful is true it waits for running
// requests to finish.
func (r *Remote) Shutdown(graceful bool) {
	if graceful {
		// TODO: need more graceful
		r.edpReader.suspend(true)
		r.edpManager.stop()

		// For some reason GRPC doesn't want to stop
		// Setup timeout as workaround but need to figure out in the future.
		// TODO: grpc not stopping
		c := make(chan bool, 1)
		go func() {
			r.s.GracefulStop()
			c <- true
		}()

		select {
		case <-c:
			r.Logger().Info("Stopped Proto.Actor server")
		case <-time.After(time.Second * 10):
			r.s.Stop()
			r.Logger().Info("Stopped Proto.Actor server", slog.String("err", "timeout"))
		}
	} else {
		r.s.Stop()
		r.Logger().Info("Killed Proto.Actor server")
	}
}

// SendMessage delivers the given message to the target PID using remoting.
func (r *Remote) SendMessage(pid *actor.PID, header actor.ReadonlyMessageHeader, message interface{}, sender *actor.PID, serializerID int32) {
	rd := &remoteDeliver{
		header:       header,
		message:      message,
		sender:       sender,
		target:       pid,
		serializerID: serializerID,
	}
	r.edpManager.remoteDeliver(rd)
}

// Logger returns the logger used by the Remote extension.
func (r *Remote) Logger() *slog.Logger {
	return r.actorSystem.Logger()
}
