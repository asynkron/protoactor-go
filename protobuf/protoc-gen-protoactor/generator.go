package main

import (
	"bytes"
	"strings"
	"text/template"

	"github.com/gogo/protobuf/protoc-gen-gogo/generator"
)

type gorelans struct {
	*generator.Generator
	generator.PluginImports
	atleastOne bool
	localName  string
	overwrite  bool
}

func NewGorleans() *gorelans {
	return &gorelans{}
}

func (p *gorelans) Name() string {
	return "gorelans"
}

func (p *gorelans) Overwrite() {
	p.overwrite = true
}

func (p *gorelans) Init(g *generator.Generator) {
	p.Generator = g
}

// GenerateImports generates the import declaration for this file.
func (g *gorelans) GenerateImports(file *generator.FileDescriptor) {
	if len(file.FileDescriptorProto.Service) == 0 {
		return
	}

	g.PrintImport("errors", "errors")
	g.PrintImport("log", "log")
	g.PrintImport("actor", "github.com/AsynkronIT/protoactor-go/actor")
	g.PrintImport("remote", "github.com/AsynkronIT/protoactor-go/remote")
	g.PrintImport("cluster", "github.com/AsynkronIT/protoactor-go/cluster")

	g.P()
}

func (p *gorelans) Generate(file *generator.FileDescriptor) {

	pkg := ProtoAst(file)

	t := template.New("hello template")
	t, _ = t.Parse(code)

	var doc bytes.Buffer
	t.Execute(&doc, pkg)
	s := doc.String()

	p.localName = generator.FileName(file)
	p.P(s)
}

func removePackagePrefix(name string, pname string) string {
	return strings.Replace(name, "."+pname+".", "", 1)
}

// func init() {
// 	generator.RegisterPlugin(NewGorleans())
// }
