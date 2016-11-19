package main

import "fmt"

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

func main() {
	g := GetHelloGrain("123")                             //typed factory to get grain
	res := <-g.SayHelloChan(&HelloRequest{Name: "Roger"}) //async wrapper func over SayHello
	fmt.Println(res)
}
