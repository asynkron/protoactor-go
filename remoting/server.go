package remoting

import (
	"log"
	"net"

	"github.com/rogeralsing/gam/actor"
	"google.golang.org/grpc"
)

type server struct{}

func (s *server) Receive(stream Remoting_ReceiveServer) error {
	for {
		batch, err := stream.Recv()
		if err != nil {
			return err
		}
		for _, envelope := range batch.Envelopes {
			pid := envelope.Target
			message := UnpackMessage(envelope)
			pid.Tell(message)
		}
	}
}

func StartServer(host string, options ...func(*RemotingConfig)) {

	lis, err := net.Listen("tcp", host)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	config := defaultRemoteConfig()
	for _, option := range options {
		option(config)
	}

	host = lis.Addr().String()
	log.Printf("Host is %v", host)
	actor.ProcessRegistry.RegisterHostResolver(remoteHandler)
	actor.ProcessRegistry.Host = host
	props := actor.
		FromProducer(newEndpointManager(config)).
		WithMailbox(actor.NewBoundedMailbox(1000, 100000))

	endpointManagerPID = actor.Spawn(props)

	s := grpc.NewServer(config.serverOptions...)
	RegisterRemotingServer(s, &server{})
	log.Printf("Starting GAM server on %v.", host)
	go s.Serve(lis)
}

func defaultRemoteConfig() *RemotingConfig {
	return &RemotingConfig{
		dialOptions: []grpc.DialOption{grpc.WithInsecure()},
		batchSize:   200,
	}
}

func WithBatchSize(batchSize int) func(*RemotingConfig) {
	return func(config *RemotingConfig) {
		config.batchSize = batchSize
	}
}

func WithDialOptions(options ...grpc.DialOption) func(*RemotingConfig) {
	return func(config *RemotingConfig) {
		config.dialOptions = options
	}
}

func WithServerOptions(options ...grpc.ServerOption) func(*RemotingConfig) {
	return func(config *RemotingConfig) {
		config.serverOptions = options
	}
}

func WithCallOptions(options ...grpc.CallOption) func(*RemotingConfig) {
	return func(config *RemotingConfig) {
		config.callOptions = options
	}
}

type RemotingConfig struct {
	serverOptions []grpc.ServerOption
	callOptions   []grpc.CallOption
	dialOptions   []grpc.DialOption
	batchSize     int
}
