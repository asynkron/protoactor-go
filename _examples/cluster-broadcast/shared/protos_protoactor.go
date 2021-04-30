package shared

import (
	"errors"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/cluster"
	"github.com/gogo/protobuf/proto"
)

var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

var system *actor.ActorSystem
// SetCluster pass created cluster
func SetSystem(sys *actor.ActorSystem) {
	system = sys
}

var c *cluster.Cluster
// SetCluster pass created cluster
func SetCluster(cluster *cluster.Cluster) {
	c = cluster
}

var xCalculatorFactory func() Calculator

// CalculatorFactory produces a Calculator
func CalculatorFactory(factory func() Calculator) {
	xCalculatorFactory = factory
}

// GetCalculatorGrain instantiates a new CalculatorGrain with given ID
func GetCalculatorGrain(id string) *CalculatorGrain {
	return &CalculatorGrain{ID: id}
}

// Calculator interfaces the services available to the Calculator
type Calculator interface {
	Init(id string)
	Terminate()

	Add(*NumberRequest, cluster.GrainContext) (*CountResponse, error)

	Subtract(*NumberRequest, cluster.GrainContext) (*CountResponse, error)

	GetCurrent(*Noop, cluster.GrainContext) (*CountResponse, error)
}

// CalculatorGrain holds the base data for the CalculatorGrain
type CalculatorGrain struct {
	ID string
}

// Add requests the execution on to the cluster using default options
func (g *CalculatorGrain) Add(r *NumberRequest) (*CountResponse, error) {
	return g.AddWithOpts(r, cluster.DefaultGrainCallOptions(c))
}

// AddWithOpts requests the execution on to the cluster
func (g *CalculatorGrain) AddWithOpts(r *NumberRequest, opts ...*cluster.GrainCallOptions) (*CountResponse, error) {
	bytes, err := proto.Marshal(r)
	if err != nil {
		return nil, err
	}
	request := &cluster.GrainRequest{MethodIndex: 0, MessageData: bytes}
	response, err := c.Call(g.ID, "Calculator", request, opts...)
	if err != nil {
		return nil, err
	}
	switch msg := response.(type) {
	case *cluster.GrainResponse:
		result := &CountResponse{}
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

// AddChan allows to use a channel to execute the method using default options
func (g *CalculatorGrain) AddChan(r *NumberRequest) (<-chan *CountResponse, <-chan error) {
	return g.AddChanWithOpts(r, cluster.DefaultGrainCallOptions(c))
}

// AddChanWithOpts allows to use a channel to execute the method
func (g *CalculatorGrain) AddChanWithOpts(r *NumberRequest, opts *cluster.GrainCallOptions) (<-chan *CountResponse, <-chan error) {
	c := make(chan *CountResponse)
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

// Subtract requests the execution on to the cluster using default options
func (g *CalculatorGrain) Subtract(r *NumberRequest) (*CountResponse, error) {
	return g.SubtractWithOpts(r, cluster.DefaultGrainCallOptions(c))
}

// SubtractWithOpts requests the execution on to the cluster
func (g *CalculatorGrain) SubtractWithOpts(r *NumberRequest, opts ...*cluster.GrainCallOptions) (*CountResponse, error) {
	bytes, err := proto.Marshal(r)
	if err != nil {
		return nil, err
	}
	request := &cluster.GrainRequest{MethodIndex: 1, MessageData: bytes}
	response, err := c.Call(g.ID, "Calculator", request, opts...)
	if err != nil {
		return nil, err
	}
	switch msg := response.(type) {
	case *cluster.GrainResponse:
		result := &CountResponse{}
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

// SubtractChan allows to use a channel to execute the method using default options
func (g *CalculatorGrain) SubtractChan(r *NumberRequest) (<-chan *CountResponse, <-chan error) {
	return g.SubtractChanWithOpts(r, cluster.DefaultGrainCallOptions(c))
}

// SubtractChanWithOpts allows to use a channel to execute the method
func (g *CalculatorGrain) SubtractChanWithOpts(r *NumberRequest, opts *cluster.GrainCallOptions) (<-chan *CountResponse, <-chan error) {
	c := make(chan *CountResponse)
	e := make(chan error)
	go func() {
		res, err := g.SubtractWithOpts(r, opts)
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

// GetCurrent requests the execution on to the cluster using default options
func (g *CalculatorGrain) GetCurrent(r *Noop) (*CountResponse, error) {
	return g.GetCurrentWithOpts(r, cluster.DefaultGrainCallOptions(c))
}

// GetCurrentWithOpts requests the execution on to the cluster
func (g *CalculatorGrain) GetCurrentWithOpts(r *Noop, opts ...*cluster.GrainCallOptions) (*CountResponse, error) {
	bytes, err := proto.Marshal(r)
	if err != nil {
		return nil, err
	}
	request := &cluster.GrainRequest{MethodIndex: 2, MessageData: bytes}
	response, err := c.Call(g.ID, "Calculator", request, opts...)
	if err != nil {
		return nil, err
	}
	switch msg := response.(type) {
	case *cluster.GrainResponse:
		result := &CountResponse{}
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

// GetCurrentChan allows to use a channel to execute the method using default options
func (g *CalculatorGrain) GetCurrentChan(r *Noop) (<-chan *CountResponse, <-chan error) {
	return g.GetCurrentChanWithOpts(r, cluster.DefaultGrainCallOptions(c))
}

// GetCurrentChanWithOpts allows to use a channel to execute the method
func (g *CalculatorGrain) GetCurrentChanWithOpts(r *Noop, opts *cluster.GrainCallOptions) (<-chan *CountResponse, <-chan error) {
	c := make(chan *CountResponse)
	e := make(chan error)
	go func() {
		res, err := g.GetCurrentWithOpts(r, opts)
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

// CalculatorActor represents the actor structure
type CalculatorActor struct {
	inner   Calculator
	Timeout *time.Duration
}

// Receive ensures the lifecycle of the actor for the received message
func (a *CalculatorActor) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *actor.Started:
		a.inner = xCalculatorFactory()
		id := ctx.Self().Id
		a.inner.Init(id[7:]) // skip "remote$"
		if a.Timeout != nil {
			ctx.SetReceiveTimeout(*a.Timeout)
		}
	case *actor.ReceiveTimeout:
		a.inner.Terminate()
		system.Root.PoisonFuture(ctx.Self()).Wait()

	case actor.AutoReceiveMessage: // pass
	case actor.SystemMessage: // pass

	case *cluster.GrainRequest:
		switch msg.MethodIndex {

		case 0:
			req := &NumberRequest{}
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

		case 1:
			req := &NumberRequest{}
			err := proto.Unmarshal(msg.MessageData, req)
			if err != nil {
				log.Fatalf("[GRAIN] proto.Unmarshal failed %v", err)
			}
			r0, err := a.inner.Subtract(req, ctx)
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
			req := &Noop{}
			err := proto.Unmarshal(msg.MessageData, req)
			if err != nil {
				log.Fatalf("[GRAIN] proto.Unmarshal failed %v", err)
			}
			r0, err := a.inner.GetCurrent(req, ctx)
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

var xTrackerFactory func() Tracker

// TrackerFactory produces a Tracker
func TrackerFactory(factory func() Tracker) {
	xTrackerFactory = factory
}

// GetTrackerGrain instantiates a new TrackerGrain with given ID
func GetTrackerGrain(id string) *TrackerGrain {
	return &TrackerGrain{ID: id}
}

// Tracker interfaces the services available to the Tracker
type Tracker interface {
	Init(id string)
	Terminate()

	RegisterGrain(*RegisterMessage, cluster.GrainContext) (*Noop, error)

	DeregisterGrain(*RegisterMessage, cluster.GrainContext) (*Noop, error)

	BroadcastGetCounts(*Noop, cluster.GrainContext) (*TotalsResponse, error)
}

// TrackerGrain holds the base data for the TrackerGrain
type TrackerGrain struct {
	ID string
}

// RegisterGrain requests the execution on to the cluster using default options
func (g *TrackerGrain) RegisterGrain(r *RegisterMessage) (*Noop, error) {
	return g.RegisterGrainWithOpts(r, cluster.DefaultGrainCallOptions(c))
}

// RegisterGrainWithOpts requests the execution on to the cluster
func (g *TrackerGrain) RegisterGrainWithOpts(r *RegisterMessage, opts ...*cluster.GrainCallOptions) (*Noop, error) {
	bytes, err := proto.Marshal(r)
	if err != nil {
		return nil, err
	}
	request := &cluster.GrainRequest{MethodIndex: 0, MessageData: bytes}
	response, err := c.Call(g.ID, "Tracker", request, opts...)
	if err != nil {
		return nil, err
	}
	switch msg := response.(type) {
	case *cluster.GrainResponse:
		result := &Noop{}
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

// RegisterGrainChan allows to use a channel to execute the method using default options
func (g *TrackerGrain) RegisterGrainChan(r *RegisterMessage) (<-chan *Noop, <-chan error) {
	return g.RegisterGrainChanWithOpts(r, cluster.DefaultGrainCallOptions(c))
}

// RegisterGrainChanWithOpts allows to use a channel to execute the method
func (g *TrackerGrain) RegisterGrainChanWithOpts(r *RegisterMessage, opts *cluster.GrainCallOptions) (<-chan *Noop, <-chan error) {
	c := make(chan *Noop)
	e := make(chan error)
	go func() {
		res, err := g.RegisterGrainWithOpts(r, opts)
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

// DeregisterGrain requests the execution on to the cluster using default options
func (g *TrackerGrain) DeregisterGrain(r *RegisterMessage) (*Noop, error) {
	return g.DeregisterGrainWithOpts(r, cluster.DefaultGrainCallOptions(c))
}

// DeregisterGrainWithOpts requests the execution on to the cluster
func (g *TrackerGrain) DeregisterGrainWithOpts(r *RegisterMessage, opts ...*cluster.GrainCallOptions) (*Noop, error) {
	bytes, err := proto.Marshal(r)
	if err != nil {
		return nil, err
	}
	request := &cluster.GrainRequest{MethodIndex: 1, MessageData: bytes}
	response, err := c.Call(g.ID, "Tracker", request, opts...)
	if err != nil {
		return nil, err
	}
	switch msg := response.(type) {
	case *cluster.GrainResponse:
		result := &Noop{}
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

// DeregisterGrainChan allows to use a channel to execute the method using default options
func (g *TrackerGrain) DeregisterGrainChan(r *RegisterMessage) (<-chan *Noop, <-chan error) {
	return g.DeregisterGrainChanWithOpts(r, cluster.DefaultGrainCallOptions(c))
}

// DeregisterGrainChanWithOpts allows to use a channel to execute the method
func (g *TrackerGrain) DeregisterGrainChanWithOpts(r *RegisterMessage, opts *cluster.GrainCallOptions) (<-chan *Noop, <-chan error) {
	c := make(chan *Noop)
	e := make(chan error)
	go func() {
		res, err := g.DeregisterGrainWithOpts(r, opts)
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

// BroadcastGetCounts requests the execution on to the cluster using default options
func (g *TrackerGrain) BroadcastGetCounts(r *Noop) (*TotalsResponse, error) {
	return g.BroadcastGetCountsWithOpts(r, cluster.DefaultGrainCallOptions(c))
}

// BroadcastGetCountsWithOpts requests the execution on to the cluster
func (g *TrackerGrain) BroadcastGetCountsWithOpts(r *Noop, opts ...*cluster.GrainCallOptions) (*TotalsResponse, error) {
	bytes, err := proto.Marshal(r)
	if err != nil {
		return nil, err
	}
	request := &cluster.GrainRequest{MethodIndex: 2, MessageData: bytes}
	response, err := c.Call(g.ID, "Tracker", request, opts...)
	if err != nil {
		return nil, err
	}
	switch msg := response.(type) {
	case *cluster.GrainResponse:
		result := &TotalsResponse{}
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

// BroadcastGetCountsChan allows to use a channel to execute the method using default options
func (g *TrackerGrain) BroadcastGetCountsChan(r *Noop) (<-chan *TotalsResponse, <-chan error) {
	return g.BroadcastGetCountsChanWithOpts(r, cluster.DefaultGrainCallOptions(c))
}

// BroadcastGetCountsChanWithOpts allows to use a channel to execute the method
func (g *TrackerGrain) BroadcastGetCountsChanWithOpts(r *Noop, opts *cluster.GrainCallOptions) (<-chan *TotalsResponse, <-chan error) {
	c := make(chan *TotalsResponse)
	e := make(chan error)
	go func() {
		res, err := g.BroadcastGetCountsWithOpts(r, opts)
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

// TrackerActor represents the actor structure
type TrackerActor struct {
	inner   Tracker
	Timeout *time.Duration
}

// Receive ensures the lifecycle of the actor for the received message
func (a *TrackerActor) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *actor.Started:
		a.inner = xTrackerFactory()
		id := ctx.Self().Id
		a.inner.Init(id[7:]) // skip "remote$"
		if a.Timeout != nil {
			ctx.SetReceiveTimeout(*a.Timeout)
		}
	case *actor.ReceiveTimeout:
		a.inner.Terminate()
		system.Root.PoisonFuture(ctx.Self()).Wait()

	case actor.AutoReceiveMessage: // pass
	case actor.SystemMessage: // pass

	case *cluster.GrainRequest:
		switch msg.MethodIndex {

		case 0:
			req := &RegisterMessage{}
			err := proto.Unmarshal(msg.MessageData, req)
			if err != nil {
				log.Fatalf("[GRAIN] proto.Unmarshal failed %v", err)
			}
			r0, err := a.inner.RegisterGrain(req, ctx)
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
			req := &RegisterMessage{}
			err := proto.Unmarshal(msg.MessageData, req)
			if err != nil {
				log.Fatalf("[GRAIN] proto.Unmarshal failed %v", err)
			}
			r0, err := a.inner.DeregisterGrain(req, ctx)
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
			req := &Noop{}
			err := proto.Unmarshal(msg.MessageData, req)
			if err != nil {
				log.Fatalf("[GRAIN] proto.Unmarshal failed %v", err)
			}
			r0, err := a.inner.BroadcastGetCounts(req, ctx)
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
