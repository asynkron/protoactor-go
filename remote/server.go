package remote

import (
	"io/ioutil"
	"log"
	"net"
	"os"

	"github.com/AsynkronIT/protoactor-go/actor"
	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"
)

// Start the remote server
func Start(address string, options ...RemotingOption) {
	grpclog.SetLogger(log.New(ioutil.Discard, "", 0))
	lis, err := net.Listen("tcp", address)
	if err != nil {
		logerr.Printf("failed to listen: %v", err)
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
	logdbg.Printf("Starting Proto.Actor server on %v", address)
	go s.Serve(lis)
}
