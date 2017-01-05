package main

import "github.com/gogo/protobuf/vanity/command"

func main() {

	req := command.Read()
	p := NewGorleans()
	p.Overwrite()

	resp := command.GeneratePlugin(req, p, "_protoactor.go")
	command.Write(resp)
}
