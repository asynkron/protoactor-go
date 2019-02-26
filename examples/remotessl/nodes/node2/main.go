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

type node2Actor struct{}

func (a *node2Actor) Receive(context actor.Context) {
	switch message := context.Message().(type) {
	case *messages.EncryptedMessage:
		if message.Message == "ACK" {
			log.Printf("%s received ACK from %s", context.Self(), context.Sender())
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
	remote.Start("localhost:8091", sslDialOption, sslServerOption)

	// start the local actor which looks for ACK messages from node1
	context := actor.EmptyRootContext
	props := actor.PropsFromProducer(func() actor.Actor { return &node2Actor{} })
	pid, err := context.SpawnNamed(props, "node2")
	if err != nil {
		panic(err)
	}

	// send a SYN message to the remove node1
	remotePID := actor.NewPID("localhost:8090", "node1")
	log.Printf("%s sending SYN to %s", pid, remotePID)
	context.RequestWithCustomSender(remotePID, &messages.EncryptedMessage{Message: "SYN"}, pid)

	console.ReadLine()
}
