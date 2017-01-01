package shared

import "github.com/AsynkronIT/protoactor-go/cluster/grain"

//a Go struct implementing the Hello interface
type hello struct {
	grain.Grain
}

func (h *hello) SayHello(r *HelloRequest) (*HelloResponse, error) {
	return &HelloResponse{Message: "hello " + r.Name + " from " + h.ID()}, nil
}

func (*hello) Add(r *AddRequest) (*AddResponse, error) {
	return &AddResponse{Result: r.A + r.B}, nil
}

func init() {
	//apply DI and setup logic
	HelloFactory(func() Hello { return &hello{} })
}
