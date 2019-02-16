
package shared


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

	
var xHelloFactory func() Hello

var rootContext = actor.EmptyRootContext()

func HelloFactory(factory func() Hello) {
	xHelloFactory = factory
}

func GetHelloGrain(id string) *HelloGrain {
	return &HelloGrain{ID: id}
}

type Hello interface {
	Init(id string)
		
	SayHello(*HelloRequest, cluster.GrainContext) (*HelloResponse, error)
		
	Add(*AddRequest, cluster.GrainContext) (*AddResponse, error)
		
	VoidFunc(*AddRequest, cluster.GrainContext) (*Unit, error)
		
}
type HelloGrain struct {
	ID string
}

	
func (g *HelloGrain) SayHello(r *HelloRequest) (*HelloResponse, error) {
	return g.SayHelloWithOpts(r, cluster.DefaultGrainCallOptions())
}

func (g *HelloGrain) SayHelloWithOpts(r *HelloRequest, opts *cluster.GrainCallOptions) (*HelloResponse, error) {
	fun := func() (*HelloResponse, error) {
			pid, statusCode := cluster.Get(g.ID, "Hello")
			if statusCode != remote.ResponseStatusCodeOK {
				return nil, fmt.Errorf("Get PID failed with StatusCode: %v", statusCode)
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
				return nil, errors.New("Unknown response")
			}
		}
	
	var res *HelloResponse
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

func (g *HelloGrain) SayHelloChan(r *HelloRequest) (<-chan *HelloResponse, <-chan error) {
	return g.SayHelloChanWithOpts(r, cluster.DefaultGrainCallOptions())
}

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
	
func (g *HelloGrain) Add(r *AddRequest) (*AddResponse, error) {
	return g.AddWithOpts(r, cluster.DefaultGrainCallOptions())
}

func (g *HelloGrain) AddWithOpts(r *AddRequest, opts *cluster.GrainCallOptions) (*AddResponse, error) {
	fun := func() (*AddResponse, error) {
			pid, statusCode := cluster.Get(g.ID, "Hello")
			if statusCode != remote.ResponseStatusCodeOK {
				return nil, fmt.Errorf("Get PID failed with StatusCode: %v", statusCode)
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
				return nil, errors.New("Unknown response")
			}
		}
	
	var res *AddResponse
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

func (g *HelloGrain) AddChan(r *AddRequest) (<-chan *AddResponse, <-chan error) {
	return g.AddChanWithOpts(r, cluster.DefaultGrainCallOptions())
}

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
	
func (g *HelloGrain) VoidFunc(r *AddRequest) (*Unit, error) {
	return g.VoidFuncWithOpts(r, cluster.DefaultGrainCallOptions())
}

func (g *HelloGrain) VoidFuncWithOpts(r *AddRequest, opts *cluster.GrainCallOptions) (*Unit, error) {
	fun := func() (*Unit, error) {
			pid, statusCode := cluster.Get(g.ID, "Hello")
			if statusCode != remote.ResponseStatusCodeOK {
				return nil, fmt.Errorf("Get PID failed with StatusCode: %v", statusCode)
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
				return nil, errors.New("Unknown response")
			}
		}
	
	var res *Unit
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

func (g *HelloGrain) VoidFuncChan(r *AddRequest) (<-chan *Unit, <-chan error) {
	return g.VoidFuncChanWithOpts(r, cluster.DefaultGrainCallOptions())
}

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
	

type HelloActor struct {
	inner Hello
}

func (a *HelloActor) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *actor.Started:
		a.inner = xHelloFactory()
		id := ctx.Self().Id
		a.inner.Init(id[7:]) // skip "remote$"

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
			
		case 1:
			req := &AddRequest{}
			err := proto.Unmarshal(msg.MessageData, req)
			if err != nil {
				log.Fatalf("[GRAIN] proto.Unmarshal failed %v", err)
			}
			r0, err := a.inner.Add(req, ctx)
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
			
		case 2:
			req := &AddRequest{}
			err := proto.Unmarshal(msg.MessageData, req)
			if err != nil {
				log.Fatalf("[GRAIN] proto.Unmarshal failed %v", err)
			}
			r0, err := a.inner.VoidFunc(req, ctx)
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
		
		}
	default:
		log.Printf("Unknown message %v", msg)
	}
}

	


// Why has this been removed?
// This should only be done on servers of the below Kinds
// Clients should not be forced to also be servers

// func init() {
//	
//	remote.Register("Hello", actor.PropsFromProducer(func() actor.Actor {
//		return &HelloActor {}
//		})		)
//	
// }



// type hello struct {
//	cluster.Grain
// }

// func (*hello) SayHello(r *HelloRequest, cluster.GrainContext) (*HelloResponse, error) {
// 	return &HelloResponse{}, nil
// }

// func (*hello) Add(r *AddRequest, cluster.GrainContext) (*AddResponse, error) {
// 	return &AddResponse{}, nil
// }

// func (*hello) VoidFunc(r *AddRequest, cluster.GrainContext) (*Unit, error) {
// 	return &Unit{}, nil
// }



// func init() {
// 	// apply DI and setup logic

// 	HelloFactory(func() Hello { return &hello{} })

// }





