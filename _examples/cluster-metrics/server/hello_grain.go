package main

import (
	"cluster-metrics/shared"
	"log"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
)

func init() {
	// apply DI and setup logic
	shared.HelloFactory(func() shared.Hello { return &HelloGrain{} })
}

// a Go struct implementing the Hello interface
type HelloGrain struct {
	cluster.Grain
}

func (h *HelloGrain) Init(ci *cluster.ClusterIdentity, cl *cluster.Cluster) {
	h.Grain.Init(ci, cl)
	log.Printf("new grain id=%s", ci.Identity)
}

func (h *HelloGrain) Terminate() {
	log.Printf("delete grain id=%s", h.Grain.Identity())
}

func (*HelloGrain) ReceiveDefault(ctx actor.Context) {
	msg := ctx.Message()
	log.Printf("Unknown message %v", msg)
}

func (h *HelloGrain) SayHello(r *shared.HelloRequest, ctx cluster.GrainContext) (*shared.HelloResponse, error) {
	return &shared.HelloResponse{Message: "hello " + r.Name + " from " + h.Identity()}, nil
}

func (*HelloGrain) Add(r *shared.AddRequest, ctx cluster.GrainContext) (*shared.AddResponse, error) {
	return &shared.AddResponse{Result: r.A + r.B}, nil
}

func (*HelloGrain) VoidFunc(r *shared.AddRequest, ctx cluster.GrainContext) (*shared.Unit, error) {
	return &shared.Unit{}, nil
}
