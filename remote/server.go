package remote

import (
	"io/ioutil"
	slog "log"
	"net"
	"os"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"
)

// Start the remote server
func Start(address string, options ...RemotingOption) {
	grpclog.SetLogger(slog.New(ioutil.Discard, "", 0))
	lis, err := net.Listen("tcp", address)
	if err != nil {
		plog.Error("failed to listen", log.Error(err))
		os.Exit(1)
	}
	config := defaultRemoteConfig()
	for _, option := range options {
		option(config)
	}

	address = lis.Addr().String()
	actor.ProcessRegistry.RegisterAddressResolver(remoteHandler)
	actor.ProcessRegistry.Address = address

	spawnActivatorActor()
	spawnEndpointManager(config)
	subscribeEndpointManager()

	s := grpc.NewServer(config.serverOptions...)
	RegisterRemotingServer(s, &server{})
	plog.Info("Starting Proto.Actor server", log.String("address", address))
	go s.Serve(lis)
}
