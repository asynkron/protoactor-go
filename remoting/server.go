package remoting

import (
	"log"
	"net"

	"github.com/AsynkronIT/protoactor-go/actor"

	"google.golang.org/grpc"
)

//Start the remoting server
func Start(host string, options ...RemotingOption) {

	lis, err := net.Listen("tcp", host)
	if err != nil {
		log.Fatalf("[REMOTING] failed to listen: %v", err)
	}
	config := defaultRemoteConfig()
	for _, option := range options {
		option(config)
	}

	spawnActivatorActor()

	host = lis.Addr().String()
	actor.ProcessRegistry.RegisterHostResolver(remoteHandler)
	actor.ProcessRegistry.Host = host
	props := actor.
		FromProducer(newEndpointManager(config)).
		WithMailbox(actor.NewBoundedMailbox(config.endpointManagerQueueSize))

	endpointManagerPID = actor.Spawn(props)

	s := grpc.NewServer(config.serverOptions...)
	RegisterRemotingServer(s, &server{})
	log.Printf("[REMOTING] Starting GAM server on %v.", host)
	go s.Serve(lis)
}
