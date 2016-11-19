package main

import (
	"strings"

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

func (p *gorelans) Generate(file *generator.FileDescriptor) {

	p.PluginImports = generator.NewPluginImports(p.Generator)
	p.atleastOne = false
	grains := p.NewImport("github.com/AsynkronIT/gam/cluster/grains")

	p.localName = generator.FileName(file)

	for _, service := range file.GetService() {
		/*
		   var fooFactory func() Foo

		   func FooFactory(factory func() Foo) {
		   	fooFactory = factory
		   }
		*/
		serviceName := service.GetName()
		factoryFieldName := "x" + generator.CamelCase(serviceName)
		factoryFuncName := factoryFieldName + "Factory"
		grainName := serviceName + "Grain"
		p.P("var ", factoryFuncName, " func() ", serviceName)
		p.P("")
		p.P("func ", serviceName, "Factory(factory func() ", serviceName, ") {")
		p.In()
		p.P(factoryFieldName, "Factory = factory")
		p.Out()
		p.P("}")
		p.P("")
		/*
		   func GetFooGrain(id string) FooGrain {
		       fg := FooGrain{
		           inner: fooFactory(),
		       }
		       return fg
		   }
		*/
		p.P("func Get", grainName, " (id string) *", grainName, " {")
		p.In()
		p.P("return &", grainName, "{inner: ", factoryFuncName, "()}")
		p.Out()
		p.P("}")
		p.P("")

		p.P("type ", serviceName, " interface {")
		p.In()
		for _, method := range service.GetMethod() {
			methodName := method.GetName()
			inputType := removePackagePrefix(method.GetInputType(), file.PackageName())
			outputType := removePackagePrefix(method.GetOutputType(), file.PackageName())
			p.P(methodName, "(*", inputType, ") *", outputType)
		}
		p.Out()
		p.P("}")

		p.P("type ", grainName, " struct {")
		p.In()
		p.P(grains.Use(), ".GrainMixin")
		p.P("inner ", serviceName)
		p.Out()
		p.P("}")
		for _, method := range service.GetMethod() {
			p.P("")
			methodName := method.GetName()
			inputType := removePackagePrefix(method.GetInputType(), file.PackageName())
			outputType := removePackagePrefix(method.GetOutputType(), file.PackageName())
			p.In()
			p.P("func (g *", grainName, ") ", methodName, " (r *", inputType, ") *", outputType, " {")
			p.In()
			p.P("return g.inner.", methodName, "(r)")
			p.Out()
			p.P("}")
			p.Out()
			p.P("")
			p.In()
			p.P("func (g *", grainName, ") ", methodName, "Chan (r *", inputType, ") <-chan *", outputType, " {")
			p.In()
			p.P("c := make(chan *", outputType, ", 1)")
			p.P("defer close(c)")
			p.P("c <- g.inner.", methodName, "(r)")
			p.P("return c")
			p.Out()
			p.P("}")
			p.Out()

		}

	}
}

func removePackagePrefix(name string, pname string) string {
	return strings.Replace(name, "."+pname+".", "", 1)
}

func init() {
	generator.RegisterPlugin(NewGorleans())
}
