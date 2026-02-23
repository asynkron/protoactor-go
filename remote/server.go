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
// Returns nil if the extension is not registered or has an unexpected type.
//
//goland:noinspection GoUnusedExportedFunction
func GetRemote(actorSystem *actor.ActorSystem) *Remote {
	r := actorSystem.Extensions.Get(extensionID)
	if r == nil {
		return nil
	}
	remote, ok := r.(*Remote)
	if !ok {
		return nil
	}
	return remote
}

// ExtensionID returns the unique ID of the Remote extension.
func (r *Remote) ExtensionID() extensions.ExtensionID {
	return extensionID
}

// BlockList returns the list of blocked members.
func (r *Remote) BlockList() *BlockList { return r.blocklist }

// Start the remote server.
func (r *Remote) Start() error {
	grpclog.SetLoggerV2(grpclog.NewLoggerV2(io.Discard, io.Discard, io.Discard))
	lis, err := net.Listen("tcp", r.config.Address())
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
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
	if err := r.edpManager.start(); err != nil {
		lis.Close()
		return fmt.Errorf("failed to start endpoint manager: %w", err)
	}

	r.s = grpc.NewServer(r.config.ServerOptions...)
	r.edpReader = newEndpointReader(r)
	RegisterRemotingServer(r.s, r.edpReader)
	r.Logger().Info("Starting Proto.Actor server", slog.String("address", address))
	go func() {
		if err := r.s.Serve(lis); err != nil {
			r.Logger().Error("gRPC server stopped", slog.Any("error", err))
		}
	}()

	return nil
}

// Shutdown stops the remote server. If graceful is true it waits for running
// requests to finish.
func (r *Remote) Shutdown(graceful bool) {
	if graceful {
		// Phase 1: Stop processing new messages on incoming streams.
		// Messages already received and being processed are unaffected;
		// GracefulStop in Phase 2 drains those.
		r.edpReader.suspend(true)

		// Phase 2: Drain in-flight RPCs with timeout
		done := make(chan struct{})
		go func() {
			r.s.GracefulStop()
			close(done)
		}()

		select {
		case <-done:
			r.Logger().Info("gRPC server stopped gracefully")
		case <-time.After(r.config.ShutdownTimeout):
			r.Logger().Warn("gRPC graceful shutdown timed out, forcing stop",
				slog.Duration("timeout", r.config.ShutdownTimeout))
			// Stop is synchronous — waits for all RPCs to terminate before
			// returning, so Phase 3 ordering is preserved on the timeout path.
			r.s.Stop()
		}

		// Phase 3: Stop endpoint management after gRPC is done
		r.edpManager.stop()
	} else {
		r.s.Stop()
		r.edpManager.stop()
		r.Logger().Info("gRPC server force-stopped")
	}
}

// SendMessage delivers the given message to the target PID using remoting.
func (r *Remote) SendMessage(pid *actor.PID, header actor.ReadonlyMessageHeader, message any, sender *actor.PID, serializerID int32) {
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
