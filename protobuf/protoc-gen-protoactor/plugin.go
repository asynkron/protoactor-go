package main

import (
	"bytes"
	"fmt"
	"go/format"
	"strings"
	"text/template"

	google_protobuf "github.com/gogo/protobuf/protoc-gen-gogo/descriptor"
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

func (p *grainGenerator) Generate(file *google_protobuf.FileDescriptorProto) string {

	pkg := ProtoAst(file)

	t := template.New("grain")
	t, _ = t.Parse(code)

	var doc bytes.Buffer
	t.Execute(&doc, pkg)
	s := doc.String()

	return s
}

func removePackagePrefix(name string, pname string) string {
	return strings.Replace(name, "."+pname+".", "", 1)
}

func (p *grainGenerator) GenerateCode(req *plugin.CodeGeneratorRequest, filenameSuffix string, goFmt bool) *plugin.CodeGeneratorResponse {

	response := &plugin.CodeGeneratorResponse{}
	for _, f := range req.GetProtoFile() {
		s := p.Generate(f)
		fileName := f.GetName() + "_actor.go"
		r := &plugin.CodeGeneratorResponse_File{
			Content: &s,
			Name:    &fileName,
		}

		response.File = append(response.File, r)
	}

	return response
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
