package main

import (
	"bytes"
	"fmt"
	"go/format"
	"strings"
	"text/template"

	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/protoc-gen-gogo/generator"
	plugin "github.com/gogo/protobuf/protoc-gen-gogo/plugin"
)

type grainGenerator struct {
	*generator.Generator
	generator.PluginImports
	atleastOne bool
	localName  string
	overwrite  bool
}

func newGrainGenerator() *grainGenerator {
	return &grainGenerator{}
}

func (p *grainGenerator) Name() string {
	return "gorelans"
}

func (p *grainGenerator) Overwrite() {
	p.overwrite = true
}

func (p *grainGenerator) Init(g *generator.Generator) {
	p.Generator = g
}

// GenerateImports generates the import declaration for this file.
func (p *grainGenerator) GenerateImports(file *generator.FileDescriptor) {
	if len(file.FileDescriptorProto.Service) == 0 {
		return
	}

	p.PrintImport("errors", "errors")
	p.PrintImport("log", "log")
	p.PrintImport("actor", "github.com/AsynkronIT/protoactor-go/actor")
	p.PrintImport("remote", "github.com/AsynkronIT/protoactor-go/remote")
	p.PrintImport("cluster", "github.com/AsynkronIT/protoactor-go/cluster")

	p.P()
}

func (p *grainGenerator) Generate(file *generator.FileDescriptor) {

	pkg := ProtoAst(file)

	t := template.New("grain")
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

func (*grainGenerator) GenerateCode(req *plugin.CodeGeneratorRequest, p generator.Plugin, filenameSuffix string, goFmt bool) *plugin.CodeGeneratorResponse {
	g := generator.New()
	g.Request = req
	if len(g.Request.FileToGenerate) == 0 {
		g.Fail("no files to generate")
	}

	g.CommandLineParameters(g.Request.GetParameter())

	g.WrapTypes()
	g.SetPackageNames()
	g.BuildTypeNameMap()
	g.GeneratePlugin(p)

	for i := 0; i < len(g.Response.File); i++ {
		g.Response.File[i].Name = proto.String(
			strings.Replace(*g.Response.File[i].Name, ".pb.go", filenameSuffix, -1),
		)
	}
	if goFmt {
		if err := goformat(g.Response); err != nil {
			g.Error(err)
		}
	}
	return g.Response
}

func goformat(resp *plugin.CodeGeneratorResponse) error {
	for i := 0; i < len(resp.File); i++ {
		formatted, err := format.Source([]byte(resp.File[i].GetContent()))
		if err != nil {
			return fmt.Errorf("go format error: %v", err)
		}
		fmts := string(formatted)
		resp.File[i].Content = &fmts
	}
	return nil
}
