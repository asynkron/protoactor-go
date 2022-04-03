package main

import (
	"cluster-grain/shared"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
)

func init() {
	// apply DI and setup logic
	shared.HelloFactory(func() shared.Hello { return &HelloGrain{} })
}

type HelloGrain struct {
	cluster.Grain
}

func (h HelloGrain) Terminate() {
}

func (h HelloGrain) ReceiveDefault(ctx actor.Context) {
}

func (h HelloGrain) SayHello(request *shared.HelloRequest, context cluster.GrainContext) (*shared.HelloResponse, error) {

	return &shared.HelloResponse{Message: "Hello " + request.Name}, nil
}
