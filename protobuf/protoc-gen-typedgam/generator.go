package main

import (
	"bytes"
	"log"
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

func (p *gorelans) AddErrorReturn() {
	p.P("if err != nil {")
	p.In()
	p.P(`return err`)
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
var grain generator.Single
var actor generator.Single
var errors generator.Single

func (p *gorelans) Generate(file *generator.FileDescriptor) {

	pkg := &ProtoFile{}
	pkg.PackageName = file.PackageName()
	messages := make(map[string]*ProtoMessage)
	for _, message := range file.GetMessageType() {
		m := &ProtoMessage{}
		m.Name = message.GetName()
		pkg.Messages = append(pkg.Messages, m)
		messages[m.Name] = m
	}

	for _, service := range file.GetService() {
		s := &ProtoService{}
		s.Name = service.GetName()
		pkg.Services = append(pkg.Services, s)

		for _, method := range service.GetMethod() {
			m := &ProtoMethod{}
			m.Name = method.GetName()
			//		m.InputStream = *method.ClientStreaming
			//		m.OutputStream = *method.ServerStreaming
			input := removePackagePrefix(method.GetInputType(), pkg.PackageName)
			output := removePackagePrefix(method.GetOutputType(), pkg.PackageName)
			m.Input = messages[input]
			m.Output = messages[output]
			s.Methods = append(s.Methods, m)
		}
	}

	t := template.New("hello template")
	t, _ = t.Parse(`

{{ range $service := .Services}}	
var x{{ $service.Name }}Factory func() {{ $service.Name }}

func {{ $service.Name }}Factory(factory func() {{ $service.Name }}) {
	x{{ $service.Name }}Factory = factory
}

func Get{{ $service.Name }}Grain(id string) *{{ $service.Name }}Grain {
	return &{{ $service.Name }}Grain{Id: id}
}

type {{ $service.Name }} interface {
	{{ range $method := $service.Methods}}	
	{{ $method.Name }}(*{{ $method.Input.Name }}) (*{{ $method.Output.Name }}, error)
	{{ end }}	
}
type {{ $service.Name }}Grain struct {
	Id string
}

{{ range $method := $service.Methods}}	
func (g *{{ $service.Name }}Grain) {{ $method.Name }}(r *{{ $method.Input.Name }}, options ...grain.GrainCallOption) (*{{ $method.Output.Name }}, error) {
	conf := grain.ApplyGrainCallOptions(options)
	var res *{{ $method.Output.Name }}
	var err error
	for i := 0; i < conf.RetryCount; i++ {
		err = func() error {
			pid, err := cluster.Get(g.Id, "{{ $service.Name }}")
			if err != nil {
				return err
			}
			bytes, err := proto.Marshal(r)
			if err != nil {
				return err
			}
			gr := &cluster.GrainRequest{Method: "{{ $method.Name }}", MessageData: bytes}
			r0 := pid.RequestFuture(gr, conf.Timeout)
			r1, err := r0.Result()
			if err != nil {
				return err
			}
			switch r2 := r1.(type) {
			case *cluster.GrainResponse:
				r3 := &{{ $method.Output.Name }}{}
				err = proto.Unmarshal(r2.MessageData, r3)
				if err != nil {
					return err
				}
				res = r3
				return nil
			case *cluster.GrainErrorResponse:
				return errors.New(r2.Err)
			default:
				return errors.New("Unknown response")
			}
		}()
		if err == nil {
			return res, nil
		}
	}
	return nil, err
}

func (g *{{ $service.Name }}Grain) {{ $method.Name }}Chan(r *{{ $method.Input.Name }}, options ...grain.GrainCallOption) (<-chan *{{ $method.Output.Name }}, <-chan error) {
	c := make(chan *{{ $method.Output.Name }})
	e := make(chan error)
	go func() {
		defer close(c)
		defer close(e)
		res, err := g.{{ $method.Name }}(r, options...)
		if err != nil {
			e <- err
		} else {
			c <- res
		}
	}()
	return c, e
}
{{ end }}	

type {{ $service.Name }}Actor struct {
	inner {{ $service.Name }}
}

func (a *{{ $service.Name }}Actor) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *cluster.GrainRequest:
		switch msg.Method {
		{{ range $method := $service.Methods}}	
		case "{{ $method.Name }}":
			req := &{{ $method.Input.Name }}{}
			err := proto.Unmarshal(msg.MessageData, req)
			if err != nil {
				log.Fatalf("[GRAIN] proto.Unmarshal failed %v", err)
			}
			r0, err := a.inner.{{ $method.Name }}(req)
			if err == nil {
				bytes, err := proto.Marshal(r0)
				if err != nil {
					log.Fatalf("[GRAIN] proto.Marshal failed %v", err)
				}
				resp := &cluster.GrainResponse{MessageData: bytes}
				ctx.Respond(resp)
			} else {
				resp := &cluster.GrainErrorResponse{Err: err.Error()}
				ctx.Respond(resp)
			}
		{{ end }}
		}
	default:
		log.Printf("Unknown message %v", msg)
	}
}

{{ end }}	


func init() {
	{{ range $service := .Services}}
	cluster.Register("{{ $service.Name }}", actor.FromProducer(func() actor.Actor { return &{{ $service.Name }}Actor{inner: x{{ $service.Name }}Factory()} }))
	{{ end }}	
}

	
	`)

	var doc bytes.Buffer
	t.Execute(&doc, pkg)
	s := doc.String()
	log.Println(s)

	p.PluginImports = generator.NewPluginImports(p.Generator)
	p.atleastOne = false
	logg = p.NewImport("log")
	time = p.NewImport("time")
	errors = p.NewImport("errors")
	cluster = p.NewImport("github.com/AsynkronIT/gam/cluster")
	grain = p.NewImport("github.com/AsynkronIT/gam/cluster/grain")
	actor = p.NewImport("github.com/AsynkronIT/gam/actor")

	p.localName = generator.FileName(file)

	for _, service := range file.GetService() {

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
		p.P("return &", grainName, "{Id:id}")
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
			p.P("func (g *", grainName, ") ", methodName, " (r *", inputType, ", options ...", grain.Use(), ".GrainCallOption) (*", outputType, ", error) {")
			p.In()
			p.P(`conf := `, grain.Use(), `.ApplyGrainCallOptions(options)`)
			p.P(`var res *`, outputType)
			p.P(`var err error`)
			p.P(`for i := 0; i < conf.RetryCount; i++ {`)
			p.In()
			p.P(`err = func() error {`)
			p.In()
			p.P(`pid := `, cluster.Use(), `.Get(g.Id,"`, serviceName, `")`)
			p.P(`bytes, err := proto.Marshal(r)`)
			p.AddErrorReturn()
			p.P(`gr := &`, cluster.Use(), `.GrainRequest{Method: "`, methodName, `", MessageData: bytes}`)
			p.P("r0 := pid.RequestFuture(gr, conf.Timeout)")
			p.P("r1, err := r0.Result()")
			p.AddErrorReturn()
			p.P(`switch r2 := r1.(type) {`)
			p.P("case *", cluster.Use(), ".GrainResponse:")
			p.In()
			p.P("r3 := &", outputType, "{}")
			p.P("err = proto.Unmarshal(r2.MessageData, r3)")
			p.AddErrorReturn()
			p.P("res = r3")
			p.P("return nil")
			p.Out()
			p.P("case *", cluster.Use(), ".GrainErrorResponse:")
			p.In()
			p.P("return ", errors.Use(), ".New(r2.Err)")
			p.Out()
			p.P("default:")
			p.In()
			p.P(`return errors.New("Unknown response")`)
			p.Out()
			p.P("}")

			p.Out()
			p.P("}()")
			p.P(`if err == nil {`)
			p.In()
			p.P(`return res, nil`)
			p.Out()
			p.P("}")
			p.Out()
			p.P("}")
			p.P(`return nil, err`)
			p.Out()
			p.P("}")
			p.Out()
			p.P("")
			p.In()
			p.P("func (g *", grainName, ") ", methodName, "Chan (r *", inputType, ", options ...", grain.Use(), ".GrainCallOption) (<-chan *", outputType, ", <-chan error) {")
			p.In()
			p.P("c := make(chan *", outputType, ")")
			p.P("e := make(chan error)")
			p.P("go func() {")
			p.In()
			p.P("defer close(c)")
			p.P("defer close(e)")
			p.P("res, err := g.", methodName, "(r, options...)")
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
