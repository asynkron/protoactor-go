package main

import (
	"fmt"

	"github.com/asynkron/protoactor-go/cluster"
)

type HelloGrain struct{}

func NewHelloGrain() Hello {
	return &HelloGrain{}
}

// Init implements Hello.
func (g *HelloGrain) Init(ctx cluster.GrainContext) {
	ctx.Logger().Info("HelloGrain Init")
}

// ReceiveDefault implements Hello.
func (g *HelloGrain) ReceiveDefault(ctx cluster.GrainContext) {
	ctx.Logger().Info("HelloGrain ReceiveDefault")
}

// Terminate implements Hello.
func (g *HelloGrain) Terminate(ctx cluster.GrainContext) {
	ctx.Logger().Info("HelloGrain Terminate")
}

// Hello implements Hello.
func (*HelloGrain) Hello(req *HelloRequest, ctx cluster.GrainContext) (*HelloResponse, error) {
	if req.Name == "user-not-found" {
		return nil, ErrUserNotFound("not found")
	}

	if req.Name == "normal-error" {
		return nil, fmt.Errorf("normal error")
	}

	return &HelloResponse{Message: "Hello " + req.Name}, nil
}

// Reenterable implements Hello.
func (*HelloGrain) Reenterable(req *ReenterableRequest, respond func(*ReenterableResponse), onError func(error), ctx cluster.GrainContext) error {
	panic("unimplemented")
}
