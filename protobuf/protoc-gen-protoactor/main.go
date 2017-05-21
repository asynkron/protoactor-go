package main

import "github.com/gogo/protobuf/vanity/command"

func main() {

	req := command.Read()
	p := newGrainGenerator()
	p.Overwrite()
	resp := p.GenerateCode(req, p, "_protoactor.go", true)
	command.Write(resp)
}
