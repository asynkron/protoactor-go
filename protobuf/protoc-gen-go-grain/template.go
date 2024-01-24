package main

import (
	"bytes"
	_ "embed"
	"strings"
	"text/template"

	"github.com/asynkron/protoactor-go/protobuf/protoc-gen-go-grain/options"
)

//go:embed templates/grain.tmpl
var grainTemplate string

//go:embed templates/error.tmpl
var errorTemplate string

type serviceDesc struct {
	Name    string // Greeter
	Methods []*methodDesc
}

type methodDesc struct {
	Name    string
	Input   string
	Output  string
	Index   int
	Options *options.MethodOptions
}

type errorDesc struct {
	Name       string
	Value      string
	CamelValue string
	Comment    string
	HasComment bool
}

type errorsWrapper struct {
	Errors []*errorDesc
}

func (es *errorsWrapper) execute() string {
	buf := new(bytes.Buffer)
	tmpl, err := template.New("error").Parse(strings.TrimSpace(errorTemplate))
	if err != nil {
		panic(err)
	}
	if err := tmpl.Execute(buf, es); err != nil {
		panic(err)
	}

	return strings.Trim(buf.String(), "\r\n")
}

func (s *serviceDesc) execute() string {
	buf := new(bytes.Buffer)
	tmpl, err := template.New("grain").Parse(strings.TrimSpace(grainTemplate))
	if err != nil {
		panic(err)
	}
	if err := tmpl.Execute(buf, s); err != nil {
		panic(err)
	}

	return strings.Trim(buf.String(), "\r\n")
}
