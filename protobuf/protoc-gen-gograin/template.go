package main

const code = `{{ if .Services }}
package {{.PackageName}}

import (
	"errors"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/otherview/protoactor-go/actor"
	"github.com/otherview/protoactor-go/cluster"
	"github.com/otherview/protoactor-go/remote"
	"github.com/gogo/protobuf/proto"
)

var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

var rootContext = actor.EmptyRootContext
{{ range $service := .Services}}	
var x{{ $service.Name }}Factory func() {{ $service.Name }}

// {{ $service.Name }}Factory produces a {{ $service.Name }}
func {{ $service.Name }}Factory(factory func() {{ $service.Name }}) {
	x{{ $service.Name }}Factory = factory
}

// Get{{ $service.Name }}Grain instantiates a new {{ $service.Name }}Grain with given ID
func Get{{ $service.Name }}Grain(id string) *{{ $service.Name }}Grain {
	return &{{ $service.Name }}Grain{ID: id}
}

// {{ $service.Name }} interfaces the services available to the {{ $service.Name }}
type {{ $service.Name }} interface {
	Init(id string)
	Terminate()
	{{ range $method := $service.Methods}}	
	{{ $method.Name }}(*{{ $method.Input.Name }}, cluster.GrainContext) (*{{ $method.Output.Name }}, error)
	{{ end }}	
}

// {{ $service.Name }}Grain holds the base data for the {{ $service.Name }}Grain
type {{ $service.Name }}Grain struct {
	ID string
}
{{ range $method := $service.Methods}}	
// {{ $method.Name }} requests the execution on to the cluster using default options
func (g *{{ $service.Name }}Grain) {{ $method.Name }}(r *{{ $method.Input.Name }}) (*{{ $method.Output.Name }}, error) {
	return g.{{ $method.Name }}WithOpts(r, cluster.DefaultGrainCallOptions())
}

// {{ $method.Name }}WithOpts requests the execution on to the cluster
func (g *{{ $service.Name }}Grain) {{ $method.Name }}WithOpts(r *{{ $method.Input.Name }}, opts *cluster.GrainCallOptions) (*{{ $method.Output.Name }}, error) {
	fun := func() (*{{ $method.Output.Name }}, error) {
			pid, statusCode := cluster.Get(g.ID, "{{ $service.Name }}")
			if statusCode != remote.ResponseStatusCodeOK && statusCode != remote.ResponseStatusCodePROCESSNAMEALREADYEXIST {
				return nil, fmt.Errorf("get PID failed with StatusCode: %v", statusCode)
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
				return nil, errors.New("unknown response")
			}
		}
	
	var res *{{ $method.Output.Name }}
	var err error
	for i := 0; i < opts.RetryCount; i++ {
		res, err = fun()
		if err == nil || err.Error() != "future: timeout" {
			return res, err
		} else if opts.RetryAction != nil {
				opts.RetryAction(i)
		}
	}
	return nil, err
}

// {{ $method.Name }}Chan allows to use a channel to execute the method using default options
func (g *{{ $service.Name }}Grain) {{ $method.Name }}Chan(r *{{ $method.Input.Name }}) (<-chan *{{ $method.Output.Name }}, <-chan error) {
	return g.{{ $method.Name }}ChanWithOpts(r, cluster.DefaultGrainCallOptions())
}

// {{ $method.Name }}ChanWithOpts allows to use a channel to execute the method
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

// {{ $service.Name }}Actor represents the actor structure
type {{ $service.Name }}Actor struct {
	inner {{ $service.Name }}
	Timeout *time.Duration
}

// Receive ensures the lifecycle of the actor for the received message
func (a *{{ $service.Name }}Actor) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *actor.Started:
		a.inner = x{{ $service.Name }}Factory()
		id := ctx.Self().Id
		a.inner.Init(id[7:]) // skip "remote$"
		if a.Timeout != nil {
			ctx.SetReceiveTimeout(*a.Timeout)
		}
	case *actor.ReceiveTimeout:
		a.inner.Terminate()
		ctx.Self().Poison()

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
				bytes, errMarshal := proto.Marshal(r0)
				if errMarshal != nil {
					log.Fatalf("[GRAIN] proto.Marshal failed %v", errMarshal)
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

{{ end}}

`
