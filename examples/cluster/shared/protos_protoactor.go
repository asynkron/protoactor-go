
package shared

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
	
var xHelloFactory func() Hello

// HelloFactory produces a Hello
func HelloFactory(factory func() Hello) {
	xHelloFactory = factory
}

// GetHelloGrain instantiates a new HelloGrain with given ID
func GetHelloGrain(id string) *HelloGrain {
	return &HelloGrain{ID: id}
}

// Hello interfaces the services available to the Hello
type Hello interface {
	Init(id string)
	Terminate()
		
	SayHello(*HelloRequest, cluster.GrainContext) (*HelloResponse, error)
		
	Add(*AddRequest, cluster.GrainContext) (*AddResponse, error)
		
	VoidFunc(*AddRequest, cluster.GrainContext) (*Unit, error)
		
}

// HelloGrain holds the base data for the HelloGrain
type HelloGrain struct {
	ID string
}
	
// SayHello requests the execution on to the cluster using default options
func (g *HelloGrain) SayHello(r *HelloRequest) (*HelloResponse, error) {
	return g.SayHelloWithOpts(r, cluster.DefaultGrainCallOptions())
}

// SayHelloWithOpts requests the execution on to the cluster
func (g *HelloGrain) SayHelloWithOpts(r *HelloRequest, opts *cluster.GrainCallOptions) (*HelloResponse, error) {
	fun := func() (*HelloResponse, error) {
			pid, statusCode := cluster.Get(g.ID, "Hello")
			if statusCode != remote.ResponseStatusCodeOK {
				return nil, fmt.Errorf("get PID failed with StatusCode: %v", statusCode)
			}
			bytes, err := proto.Marshal(r)
			if err != nil {
				return nil, err
			}
			request := &cluster.GrainRequest{MethodIndex: 0, MessageData: bytes}
			response, err := rootContext.RequestFuture(pid, request, opts.Timeout).Result()
			if err != nil {
				return nil, err
			}
			switch msg := response.(type) {
			case *cluster.GrainResponse:
				result := &HelloResponse{}
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
	
	var res *HelloResponse
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

// SayHelloChan allows to use a channel to execute the method using default options
func (g *HelloGrain) SayHelloChan(r *HelloRequest) (<-chan *HelloResponse, <-chan error) {
	return g.SayHelloChanWithOpts(r, cluster.DefaultGrainCallOptions())
}

// SayHelloChanWithOpts allows to use a channel to execute the method
func (g *HelloGrain) SayHelloChanWithOpts(r *HelloRequest, opts *cluster.GrainCallOptions) (<-chan *HelloResponse, <-chan error) {
	c := make(chan *HelloResponse)
	e := make(chan error)
	go func() {
		res, err := g.SayHelloWithOpts(r, opts)
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
	
// Add requests the execution on to the cluster using default options
func (g *HelloGrain) Add(r *AddRequest) (*AddResponse, error) {
	return g.AddWithOpts(r, cluster.DefaultGrainCallOptions())
}

// AddWithOpts requests the execution on to the cluster
func (g *HelloGrain) AddWithOpts(r *AddRequest, opts *cluster.GrainCallOptions) (*AddResponse, error) {
	fun := func() (*AddResponse, error) {
			pid, statusCode := cluster.Get(g.ID, "Hello")
			if statusCode != remote.ResponseStatusCodeOK {
				return nil, fmt.Errorf("get PID failed with StatusCode: %v", statusCode)
			}
			bytes, err := proto.Marshal(r)
			if err != nil {
				return nil, err
			}
			request := &cluster.GrainRequest{MethodIndex: 1, MessageData: bytes}
			response, err := rootContext.RequestFuture(pid, request, opts.Timeout).Result()
			if err != nil {
				return nil, err
			}
			switch msg := response.(type) {
			case *cluster.GrainResponse:
				result := &AddResponse{}
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
	
	var res *AddResponse
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

// AddChan allows to use a channel to execute the method using default options
func (g *HelloGrain) AddChan(r *AddRequest) (<-chan *AddResponse, <-chan error) {
	return g.AddChanWithOpts(r, cluster.DefaultGrainCallOptions())
}

// AddChanWithOpts allows to use a channel to execute the method
func (g *HelloGrain) AddChanWithOpts(r *AddRequest, opts *cluster.GrainCallOptions) (<-chan *AddResponse, <-chan error) {
	c := make(chan *AddResponse)
	e := make(chan error)
	go func() {
		res, err := g.AddWithOpts(r, opts)
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
	
// VoidFunc requests the execution on to the cluster using default options
func (g *HelloGrain) VoidFunc(r *AddRequest) (*Unit, error) {
	return g.VoidFuncWithOpts(r, cluster.DefaultGrainCallOptions())
}

// VoidFuncWithOpts requests the execution on to the cluster
func (g *HelloGrain) VoidFuncWithOpts(r *AddRequest, opts *cluster.GrainCallOptions) (*Unit, error) {
	fun := func() (*Unit, error) {
			pid, statusCode := cluster.Get(g.ID, "Hello")
			if statusCode != remote.ResponseStatusCodeOK {
				return nil, fmt.Errorf("get PID failed with StatusCode: %v", statusCode)
			}
			bytes, err := proto.Marshal(r)
			if err != nil {
				return nil, err
			}
			request := &cluster.GrainRequest{MethodIndex: 2, MessageData: bytes}
			response, err := rootContext.RequestFuture(pid, request, opts.Timeout).Result()
			if err != nil {
				return nil, err
			}
			switch msg := response.(type) {
			case *cluster.GrainResponse:
				result := &Unit{}
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
	
	var res *Unit
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

// VoidFuncChan allows to use a channel to execute the method using default options
func (g *HelloGrain) VoidFuncChan(r *AddRequest) (<-chan *Unit, <-chan error) {
	return g.VoidFuncChanWithOpts(r, cluster.DefaultGrainCallOptions())
}

// VoidFuncChanWithOpts allows to use a channel to execute the method
func (g *HelloGrain) VoidFuncChanWithOpts(r *AddRequest, opts *cluster.GrainCallOptions) (<-chan *Unit, <-chan error) {
	c := make(chan *Unit)
	e := make(chan error)
	go func() {
		res, err := g.VoidFuncWithOpts(r, opts)
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
	

// HelloActor represents the actor structure
type HelloActor struct {
	inner Hello
	Timeout *time.Duration
}

// Receive ensures the lifecycle of the actor for the received message
func (a *HelloActor) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *actor.Started:
		a.inner = xHelloFactory()
		id := ctx.Self().Id
		a.inner.Init(id[7:]) // skip "remote$"
		if a.Timeout != nil {
			ctx.SetReceiveTimeout(*a.Timeout)
		}
	case *actor.ReceiveTimeout:
		a.inner.Terminate()
		rootContext.PoisonFuture(ctx.Self()).Wait()

	case actor.AutoReceiveMessage: // pass
	case actor.SystemMessage: // pass

	case *cluster.GrainRequest:
		switch msg.MethodIndex {
			
		case 0:
			req := &HelloRequest{}
			err := proto.Unmarshal(msg.MessageData, req)
			if err != nil {
				log.Fatalf("[GRAIN] proto.Unmarshal failed %v", err)
			}
			r0, err := a.inner.SayHello(req, ctx)
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
			
		case 1:
			req := &AddRequest{}
			err := proto.Unmarshal(msg.MessageData, req)
			if err != nil {
				log.Fatalf("[GRAIN] proto.Unmarshal failed %v", err)
			}
			r0, err := a.inner.Add(req, ctx)
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
			
		case 2:
			req := &AddRequest{}
			err := proto.Unmarshal(msg.MessageData, req)
			if err != nil {
				log.Fatalf("[GRAIN] proto.Unmarshal failed %v", err)
			}
			r0, err := a.inner.VoidFunc(req, ctx)
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
		
		}
	default:
		log.Printf("Unknown message %v", msg)
	}
}

	



