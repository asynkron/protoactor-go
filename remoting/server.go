package remoting

import (
	"io/ioutil"
	"log"
	"net"

	"github.com/AsynkronIT/protoactor-go/actor"

	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"
)

//Start the remoting server
func Start(address string, options ...RemotingOption) {
	grpclog.SetLogger(log.New(ioutil.Discard, "", 0))
	lis, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("[REMOTING] failed to listen: %v", err)
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
	log.Printf("[REMOTING] Starting Proto.Actor server on %v", address)
	go s.Serve(lis)
}
