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
var errors generator.Single

func (p *gorelans) Generate(file *generator.FileDescriptor) {

	p.PluginImports = generator.NewPluginImports(p.Generator)
	p.atleastOne = false
	logg = p.NewImport("log")
	time = p.NewImport("time")
	errors = p.NewImport("errors")
	cluster = p.NewImport("github.com/AsynkronIT/gam/cluster")
	actor = p.NewImport("github.com/AsynkronIT/gam/actor")

	p.localName = generator.FileName(file)
	for _, message := range file.GetMessageType() {
		messageName := message.GetName()
		p.P("type ", messageName, "Future struct {")
		p.In()
		p.P("Value	*", messageName)
		p.P("Err	error")
		p.Out()
		p.P("}")
		p.P("")
	}

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
			p.P(methodName, "(*", inputType, ") (*", outputType, ", error)")
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
			p.P(`switch r2 := r1.(type) {`)
			p.P("case *", cluster.Use(), ".GrainResponse:")
			p.In()
			p.P("r3 := &", outputType, "{}")
			p.P("err = proto.Unmarshal(r2.MessageData, r3)")
			p.AddErrorReturn()
			p.P("return r3, nil")
			p.Out()
			p.P("case *", cluster.Use(), ".GrainErrorResponse:")
			p.In()
			p.P("return nil, ", errors.Use(), ".New(r2.Err)")
			p.Out()
			p.P("default:")
			p.In()
			p.P(`return nil, errors.New("Unknown response")`)
			p.Out()
			p.P("}")
			p.Out()
			p.P("}")
			p.Out()
			p.P("")
			p.In()
			p.P("func (g *", grainName, ") ", methodName, "Chan (r *", inputType, ", timeout ", time.Use(), ".Duration) <-chan *", outputType, "Future {")
			p.In()
			p.P("c := make(chan *", outputType, "Future)")
			p.P("go func() {")
			p.In()
			p.P("defer close(c)")
			p.P("res, err := g.", methodName, "(r, timeout)")
			p.P("c <- &", outputType, "Future { Value: res, Err: err}")
			p.Out()
			p.P("}()")
			p.P("return c")
			p.Out()
			p.P("}")
			p.Out()
			p.P("")
			p.In()
			p.P("func (g *", grainName, ") ", methodName, "Chan2 (r *", inputType, ", timeout ", time.Use(), ".Duration) (<-chan *", outputType, ", <-chan error) {")
			p.In()
			p.P("c := make(chan *", outputType, ")")
			p.P("e := make(chan error)")
			p.P("go func() {")
			p.In()
			p.P("defer close(c)")
			p.P("defer close(e)")
			p.P("res, err := g.", methodName, "(r, timeout)")
			p.P("if err != nil {")
			p.In()
			p.P("e <- err")
			p.Out()
			p.P("} else {")
			p.In()
			p.P("c <- res")
			p.Out()
			p.P("}")
			p.Out()
			p.P("}()")
			p.P("return c, e")
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
			p.P(`err := proto.Unmarshal(msg.MessageData, req)`)
			p.AddErrorHandler("[GRAIN] proto.Unmarshal failed %v")
			p.P(`r0, err := a.inner.`, methodName, `(req)`)
			p.P(`if err == nil {`)
			p.In()
			p.P(`bytes, err := proto.Marshal(r0)`)
			p.AddErrorHandler("[GRAIN] proto.Marshal failed %v")
			p.P(`resp := &`, cluster.Use(), `.GrainResponse{MessageData: bytes}`)
			p.P(`ctx.Respond(resp)`)
			p.Out()
			p.P(`} else {`)
			p.In()
			p.P(`resp := &github_com_AsynkronIT_gam_cluster.GrainErrorResponse{Err: err.Error()}`)
			p.P(`ctx.Respond(resp)`)
			p.Out()
			p.P(`}`)

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
