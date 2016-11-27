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

func (p *gorelans) AddErrorReturn() {
	p.P("if err != nil {")
	p.In()
	p.P(`return nil, err`)
	p.Out()
	p.P("}")
}

func (p *gorelans) AddErrorHandler(infof string) {
	p.P("if err != nil {")
	p.In()
	p.P(`log.Fatalf("`, infof, `", err)`)
	p.Out()
	p.P("}")
}

var logg generator.Single
var time generator.Single
var cluster generator.Single
var actor generator.Single

func (p *gorelans) Generate(file *generator.FileDescriptor) {

	p.PluginImports = generator.NewPluginImports(p.Generator)
	p.atleastOne = false
	logg = p.NewImport("log")
	time = p.NewImport("time")
	cluster = p.NewImport("github.com/AsynkronIT/gam/cluster")
	actor = p.NewImport("github.com/AsynkronIT/gam/actor")

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
		p.P("func Get", grainName, " (id string) *", grainName, " {")
		p.In()
		p.P("return &HelloGrain{Id:id}")
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
		p.P("Id string")
		p.Out()
		p.P("}")
		for _, method := range service.GetMethod() {
			p.P("")
			methodName := method.GetName()
			inputType := removePackagePrefix(method.GetInputType(), file.PackageName())
			outputType := removePackagePrefix(method.GetOutputType(), file.PackageName())
			p.In()
			p.P("func (g *", grainName, ") ", methodName, " (r *", inputType, ", timeout ", time.Use(), ".Duration) (*", outputType, ", error) {")
			p.In()
			p.P(`pid := `, cluster.Use(), `.Get(g.Id,"`, serviceName, `")`)
			p.P(`bytes, err := proto.Marshal(r)`)
			p.AddErrorReturn()
			p.P(`gr := &`, cluster.Use(), `.GrainRequest{Method: "`, methodName, `", MessageData: bytes}`)
			p.P("r0 := pid.RequestFuture(gr, timeout)")
			p.P("r1, err := r0.Result()")
			p.AddErrorReturn()
			p.P("r2, _ := r1.(*", cluster.Use(), ".GrainResponse)")
			p.P("r3 := &", outputType, "{}")
			p.P("err = proto.Unmarshal(r2.MessageData, r3)")
			p.AddErrorReturn()
			p.P("return r3, nil")
			p.Out()
			p.P("}")
			p.Out()
			p.P("")
			// p.In()
			// p.P("func (g *", grainName, ") ", methodName, "Chan (r *", inputType, ", timeout ", time.Use(), ".Duration) <-chan *", outputType, " {")
			// p.In()
			// p.P("c := make(chan *", outputType, ", 1)")
			// p.P("defer close(c)")
			// p.P("res, err := g.", methodName, "(r, timeout)")
			// p.P("c <- res")
			// p.P("return c")
			// p.Out()
			// p.P("}")
			// p.Out()

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
		p.P("case *", cluster.Use(), ".GrainRequest:")
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
			p.P(`r0 := a.inner.`, methodName, `(req)`)
			p.P(`bytes, err := proto.Marshal(r0)`)
			p.AddErrorHandler("[GRAIN] proto.Marshal failed %v")
			p.P(`resp := &`, cluster.Use(), `.GrainResponse{MessageData: bytes}`)
			p.P(`ctx.Respond(resp)`)

			p.Out()

		}
		p.P("}")
		p.Out()
		p.P("default:")
		p.In()
		p.P(logg.Use(), `.Printf("Unknown message %v", msg)`)
		p.Out()
		p.Out()
		p.P("}")
		p.Out()
		p.P("}")
		p.P("")
		p.P("func init() {")
		p.In()
		for _, service := range file.GetService() {
			serviceName := service.GetName()
			factoryFieldName := "x" + generator.CamelCase(serviceName)
			factoryFuncName := factoryFieldName + "Factory"
			actorName := serviceName + "Actor"
			p.P(cluster.Use(), `.Register("`, serviceName, `",`, actor.Use(), `.FromProducer(func() `, actor.Use(), `.Actor { return &`, actorName, `{inner: `, factoryFuncName, `()} }))`)
		}
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
