package main

import (
	"log"

	console "github.com/AsynkronIT/goconsole"
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/examples/remotessl/messages"
	"github.com/AsynkronIT/protoactor-go/remote"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type node1Actor struct{}

func (a *node1Actor) Receive(context actor.Context) {
	switch message := context.Message().(type) {
	case *messages.EncryptedMessage:
		if message.Message == "SYN" {
			log.Printf("%s received SYN from %s", context.Self(), context.Sender())
			context.Sender().Request(&messages.EncryptedMessage{Message: "ACK"}, context.Self())
		}
	}
}

var (
	crt = "cert/localhost.crt"
	key = "cert/localhost.key"
)

func main() {
	// get the cert and key for TLS
	serverCreds, err := credentials.NewServerTLSFromFile(crt, key)
	if err != nil {
		panic(err)
	}
	clientCreds, _ := credentials.NewClientTLSFromFile(crt, "")

	// configure and start the remote server with SSL
	sslServerOption := remote.WithServerOptions(grpc.Creds(serverCreds))
	sslDialOption := remote.WithDialOptions(grpc.WithTransportCredentials(clientCreds))
	remote.Start("localhost:8090", sslDialOption, sslServerOption)

	// start the local actor which looks for SYN messages from node2
	props := actor.FromProducer(func() actor.Actor { return &node1Actor{} })
	_, err = actor.SpawnNamed(props, "node1")
	if err != nil {
		panic(err)
	}

	console.ReadLine()
}
