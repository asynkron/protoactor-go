package main

import "fmt"

type hello struct {
}

func (*hello) SayHello(r *HelloRequest) *HelloResponse {
	return &HelloResponse{Message: "hello " + r.Name}
}

func main() {

	HelloFactory(func() Hello { return &hello{} })
	g := GetHelloGrain("123")
	res := <-g.SayHelloChan(&HelloRequest{Name: "Roger"})
	fmt.Println(res)
}
