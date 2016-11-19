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
	logg := p.NewImport("log")
	grains := p.NewImport("github.com/AsynkronIT/gam/cluster/grains")
	actor := p.NewImport("github.com/AsynkronIT/gam/actor")

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
		actorName := serviceName + "Actor"
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
		p.P("pid *", actor.Use(), ".PID")
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
			p.P(`bytes, _ := proto.Marshal(r)`)
			p.P(`gr := &`, grains.Use(), `.GrainRequest{Method: "`, methodName, `", MessageData: bytes}`)
			p.P("res,_ := g.pid.AskFuture(gr,1000)")
			p.P("return res")
			p.Out()
			p.P("}")
			p.Out()
			p.P("")
			p.In()
			p.P("func (g *", grainName, ") ", methodName, "Chan (r *", inputType, ") <-chan *", outputType, " {")
			p.In()
			p.P("c := make(chan *", outputType, ", 1)")
			p.P("defer close(c)")
			p.P("c <- g.", methodName, "(r)")
			p.P("return c")
			p.Out()
			p.P("}")
			p.Out()

		}
		p.P("")
		p.P("type ", actorName, " struct {")
		p.In()
		p.P("inner ", serviceName)
		p.Out()
		p.P("}")
		p.P("")
		p.P("func (a *", actorName, ") Receive(ctx ", actor.Use(), ".Context) {")
		p.In()
		p.P("switch msg := ctx.Message().(type) {")
		p.In()
		p.P("case *", grains.Use(), ".GrainRequest:")
		p.In()
		p.P("switch msg.Method {")
		for _, method := range service.GetMethod() {
			methodName := method.GetName()
			inputType := removePackagePrefix(method.GetInputType(), file.PackageName())
			//	outputType := removePackagePrefix(method.GetOutputType(), file.PackageName())
			p.P(`case "`, methodName, `":`)
			p.In()
			p.P(`req := &`, inputType, `{}`)
			p.P(`proto.Unmarshal(msg.MessageData, req)`)
			p.P(`a.inner.`, methodName, `(req)`)
			p.Out()
			// methodName := method.GetName()
			// inputType := removePackagePrefix(method.GetInputType(), file.PackageName())
			// outputType := removePackagePrefix(method.GetOutputType(), file.PackageName())
		}
		p.P("}")
		p.Out()
		p.P("default:")
		p.In()
		p.P(logg.Use(), `.Printf("Unknown message %v", msg)`)
		p.Out()
		/*
			case *grains.GrainRequest:
					switch msg.Method {
					case "SayHello":
						req := &HelloRequest{}
						proto.Unmarshal(msg.MessageData, req)
						a.inner.SayHello(req)

					}
				default:
					log.Printf("Unknown message %+v", msg)
		*/

		p.Out()
		p.P("}")
		p.Out()
		p.P("}")
	}
}

func removePackagePrefix(name string, pname string) string {
	return strings.Replace(name, "."+pname+".", "", 1)
}

func init() {
	generator.RegisterPlugin(NewGorleans())
}
