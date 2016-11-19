package shared

//a Go struct implementing the Hello interface
type hello struct {
}

func (*hello) SayHello(r *HelloRequest) *HelloResponse {
	return &HelloResponse{Message: "hello " + r.Name}
}

func init() {
	//apply DI and setup logic
	HelloFactory(func() Hello { return &hello{} })
}
