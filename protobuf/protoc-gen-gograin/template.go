package main

const code = `
package {{.PackageName}}


import errors "errors"
import log "log"
import actor "github.com/AsynkronIT/protoactor-go/actor"
import remote "github.com/AsynkronIT/protoactor-go/remote"
import cluster "github.com/AsynkronIT/protoactor-go/cluster"

import proto "github.com/gogo/protobuf/proto"
import fmt "fmt"
import math "math"

var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

{{ range $service := .Services}}	
var x{{ $service.Name }}Factory func() {{ $service.Name }}

var rootContext = actor.EmptyRootContext()

func {{ $service.Name }}Factory(factory func() {{ $service.Name }}) {
	x{{ $service.Name }}Factory = factory
}

func Get{{ $service.Name }}Grain(id string) *{{ $service.Name }}Grain {
	return &{{ $service.Name }}Grain{ID: id}
}

type {{ $service.Name }} interface {
	Init(id string)
	{{ range $method := $service.Methods}}	
	{{ $method.Name }}(*{{ $method.Input.Name }}, cluster.GrainContext) (*{{ $method.Output.Name }}, error)
	{{ end }}	
}
type {{ $service.Name }}Grain struct {
	ID string
}

{{ range $method := $service.Methods}}	
func (g *{{ $service.Name }}Grain) {{ $method.Name }}(r *{{ $method.Input.Name }}) (*{{ $method.Output.Name }}, error) {
	return g.{{ $method.Name }}WithOpts(r, cluster.DefaultGrainCallOptions())
}

func (g *{{ $service.Name }}Grain) {{ $method.Name }}WithOpts(r *{{ $method.Input.Name }}, opts *cluster.GrainCallOptions) (*{{ $method.Output.Name }}, error) {
	fun := func() (*{{ $method.Output.Name }}, error) {
			pid, statusCode := cluster.Get(g.ID, "{{ $service.Name }}")
			if statusCode != remote.ResponseStatusCodeOK {
				return nil, fmt.Errorf("Get PID failed with StatusCode: %v", statusCode)
			}
			bytes, err := proto.Marshal(r)
			if err != nil {
				return nil, err
			}
			request := &cluster.GrainRequest{MethodIndex: {{ $method.Index }}, MessageData: bytes}
			response, err := rootContext.RequestFuture(pid, request, opts.Timeout).Result()
			if err != nil {
				return nil, err
			}
			switch msg := response.(type) {
			case *cluster.GrainResponse:
				result := &{{ $method.Output.Name }}{}
				err = proto.Unmarshal(msg.MessageData, result)
				if err != nil {
					return nil, err
				}
				return result, nil
			case *cluster.GrainErrorResponse:
				return nil, errors.New(msg.Err)
			default:
				return nil, errors.New("Unknown response")
			}
		}
	
	var res *{{ $method.Output.Name }}
	var err error
	for i := 0; i < opts.RetryCount; i++ {
		res, err = fun()
		if err == nil {
			return res, nil
		} else {
			if opts.RetryAction != nil {
				opts.RetryAction(i)
			}
		}
	}
	return nil, err
}

func (g *{{ $service.Name }}Grain) {{ $method.Name }}Chan(r *{{ $method.Input.Name }}) (<-chan *{{ $method.Output.Name }}, <-chan error) {
	return g.{{ $method.Name }}ChanWithOpts(r, cluster.DefaultGrainCallOptions())
}

func (g *{{ $service.Name }}Grain) {{ $method.Name }}ChanWithOpts(r *{{ $method.Input.Name }}, opts *cluster.GrainCallOptions) (<-chan *{{ $method.Output.Name }}, <-chan error) {
	c := make(chan *{{ $method.Output.Name }})
	e := make(chan error)
	go func() {
		res, err := g.{{ $method.Name }}WithOpts(r, opts)
		if err != nil {
			e <- err
		} else {
			c <- res
		}
		close(c)
		close(e)
	}()
	return c, e
}
{{ end }}	

type {{ $service.Name }}Actor struct {
	inner {{ $service.Name }}
}

func (a *{{ $service.Name }}Actor) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *actor.Started:
		a.inner = x{{ $service.Name }}Factory()
		id := ctx.Self().Id
		a.inner.Init(id[7:]) // skip "remote$"

	case actor.AutoReceiveMessage: // pass
	case actor.SystemMessage: // pass

	case *cluster.GrainRequest:
		switch msg.MethodIndex {
		{{ range $method := $service.Methods}}	
		case {{ $method.Index }}:
			req := &{{ $method.Input.Name }}{}
			err := proto.Unmarshal(msg.MessageData, req)
			if err != nil {
				log.Fatalf("[GRAIN] proto.Unmarshal failed %v", err)
			}
			r0, err := a.inner.{{ $method.Name }}(req, ctx)
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


// Why has this been removed?
// This should only be done on servers of the below Kinds
// Clients should not be forced to also be servers

// func init() {
//	{{ range $service := .Services}}
//	remote.Register("{{ $service.Name }}", actor.PropsFromProducer(func() actor.Actor {
//		return &{{ $service.Name }}Actor {}
//		})		)
//	{{ end }}
// }


{{ range $service := .Services}}
// type {{ $service.PascalName }} struct {
//	cluster.Grain
// }
{{ range $method := $service.Methods}}
// func (*{{ $service.PascalName }}) {{ $method.Name }}(r *{{ $method.Input.Name }}, cluster.GrainContext) (*{{ $method.Output.Name }}, error) {
// 	return &{{ $method.Output.Name }}{}, nil
// }
{{ end }}
{{ end }}

// func init() {
// 	// apply DI and setup logic
{{ range $service := .Services}}
// 	{{ $service.Name }}Factory(func() {{ $service.Name }} { return &{{ $service.PascalName }}{} })
{{ end }}
// }





`
