package remote

import (
	"fmt"
	"io/ioutil"
	"log/slog"
	"net"
	"time"

	"github.com/asynkron/protoactor-go/extensions"

	"github.com/asynkron/protoactor-go/actor"
	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"
)

var extensionId = extensions.NextExtensionID()

type Remote struct {
	actorSystem  *actor.ActorSystem
	s            *grpc.Server
	edpReader    *endpointReader
	edpManager   *endpointManager
	config       *Config
	kinds        map[string]*actor.Props
	activatorPid *actor.PID
	blocklist    *BlockList
}

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

	actorSystem.Extensions.Register(r)

	return r
}

//goland:noinspection GoUnusedExportedFunction
func GetRemote(actorSystem *actor.ActorSystem) *Remote {
	r := actorSystem.Extensions.Get(extensionId)

	return r.(*Remote)
}

func (r *Remote) ExtensionID() extensions.ExtensionID {
	return extensionId
}

func (r *Remote) BlockList() *BlockList { return r.blocklist }

// Start the remote server
func (r *Remote) Start() {
	grpclog.SetLoggerV2(grpclog.NewLoggerV2(ioutil.Discard, ioutil.Discard, ioutil.Discard))
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
	go r.s.Serve(lis)
}

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

func (r *Remote) Logger() *slog.Logger {
	return r.actorSystem.Logger()
}
