package shared

import "github.com/otherview/protoactor-go/cluster"

// a Go struct implementing the Hello interface
type hello struct {
	cluster.Grain
}

func (*hello) Terminate()  {}

func (h *hello) SayHello(r *HelloRequest, ctx cluster.GrainContext) (*HelloResponse, error) {
	return &HelloResponse{Message: "hello " + r.Name + " from " + h.ID()}, nil
}

func (*hello) Add(r *AddRequest, ctx cluster.GrainContext) (*AddResponse, error) {
	return &AddResponse{Result: r.A + r.B}, nil
}

func (*hello) VoidFunc(r *AddRequest, ctx cluster.GrainContext) (*Unit, error) {
	return &Unit{}, nil
}

func init() {
	// apply DI and setup logic
	HelloFactory(func() Hello { return &hello{} })
}
