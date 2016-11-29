package main

const code = `

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
	cluster.Register("{{ $service.Name }}", actor.FromProducer(func() actor.Actor { 
		return &{{ $service.Name }}Actor {
			inner: x{{ $service.Name }}Factory(),
		}
		})		)
	{{ end }}	
}`
