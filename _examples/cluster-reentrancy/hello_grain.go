package main

import (
	errors "errors"

	"github.com/asynkron/protoactor-go/cluster"
)

type HelloGrain struct{}

func NewHelloGrain() Hello {
	return &HelloGrain{}
}

// Init implements Hello.
func (g *HelloGrain) Init(ctx cluster.GrainContext) {
	ctx.Logger().Info("HelloGrain Init")
}

// ReceiveDefault implements Hello.
func (g *HelloGrain) ReceiveDefault(ctx cluster.GrainContext) {
	ctx.Logger().Info("HelloGrain ReceiveDefault")
}

// InvokeService implements Hello.
func (g *HelloGrain) InvokeService(req *InvokeServiceRequest, respond func(*InvokeServiceResponse), onError func(error), ctx cluster.GrainContext) error {
	if ctx.Identity() == "1" {
		if req.Name == "Bob" {
			respond(&InvokeServiceResponse{
				Message: "Hello " + req.Name + " from " + ctx.Identity(),
			})
			return nil
		}

		client := GetHelloGrainClient(ctx.Cluster(), "1")
		f, err := client.InvokeServiceFuture(&InvokeServiceRequest{Name: "Bob"})
		ctx.ReenterAfter(f, func(resp interface{}, err error) {
			if err != nil {
				onError(err)
				return
			}

			switch msg := resp.(type) {
			case *InvokeServiceResponse:
				respond(&InvokeServiceResponse{
					Message: req.Name + " from " + ctx.Identity() + " and " + msg.Message,
				})
			case error:
				onError(msg)
			default:
				onError(errors.New("unknown response"))
			}
		})
		if err != nil {
			return err
		}
	}

	if ctx.Identity() == "2" {
		client := GetHelloGrainClient(ctx.Cluster(), "1")
		f, err := client.DoWorkFuture(&DoWorkRequest{Name: "Work1"})
		if err != nil {
			return err
		}

		ctx.ReenterAfter(f, func(resp interface{}, err error) {
			if err != nil {
				onError(err)
			}

			switch msg := resp.(type) {
			case *DoWorkResponse:
				respond(&InvokeServiceResponse{
					Message: "Hello " + req.Name + " from " + ctx.Identity() + " and " + msg.Message,
				})
			case error:
				onError(msg)
			default:
				onError(errors.New("unknown response"))
			}
		})
	}

	return nil
}

// DoWork implements Hello.
func (g *HelloGrain) DoWork(req *DoWorkRequest, ctx cluster.GrainContext) (*DoWorkResponse, error) {
	if ctx.Identity() == "1" {
		client := GetHelloGrainClient(ctx.Cluster(), "2")
		response, err := client.DoWork(&DoWorkRequest{Name: "Work2"})
		if err != nil {
			return nil, err
		}

		return &DoWorkResponse{
			Message: req.Name + " done " + response.Message,
		}, nil
	}

	if ctx.Identity() == "2" {
		return &DoWorkResponse{
			Message: req.Name + " done",
		}, nil
	}

	return nil, errors.New("unknown identity")
}

// Terminate implements Hello.
func (g *HelloGrain) Terminate(ctx cluster.GrainContext) {
	ctx.Logger().Info("HelloGrain Terminate")
}
