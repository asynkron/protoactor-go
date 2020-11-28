package shared

import (
	"log"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/cluster"
)

func init() {
	// apply DI and setup logic
	HelloFactory(func() Hello { return &HelloGrain{} })
}

// a Go struct implementing the Hello interface
type HelloGrain struct {
	cluster.Grain
}

func (h *HelloGrain) Init(id string) {
	h.Grain.Init(id)
	log.Printf("new grain id=%s", id)
}

func (h *HelloGrain) Terminate() {
	log.Printf("delete grain id=%s", h.Grain.ID())
}

func (*HelloGrain) ReceiveDefault(ctx actor.Context) {
	msg := ctx.Message()
	log.Printf("Unknown message %v", msg)
}

func (h *HelloGrain) SayHello(r *HelloRequest, ctx cluster.GrainContext) (*HelloResponse, error) {
	return &HelloResponse{Message: "hello " + r.Name + " from " + h.ID()}, nil
}

func (*HelloGrain) Add(r *AddRequest, ctx cluster.GrainContext) (*AddResponse, error) {
	return &AddResponse{Result: r.A + r.B}, nil
}

func (*HelloGrain) VoidFunc(r *AddRequest, ctx cluster.GrainContext) (*Unit, error) {
	return &Unit{}, nil
}
