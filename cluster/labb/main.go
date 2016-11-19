package main

import "fmt"

func main() {
	g := GetHelloGrain("123")
	res := <-g.SayHelloChan(&HelloRequest{Name: "Roger"})
	fmt.Println(res)
}
