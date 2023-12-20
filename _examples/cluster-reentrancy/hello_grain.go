package main

import (
	errors "errors"
	slog "log/slog"
	"os"

	"github.com/asynkron/protoactor-go/cluster"
)

type HelloGrain struct {
	logger *slog.Logger
}

func NewHelloGrain() Hello {
	return &HelloGrain{
		logger: slog.New(slog.NewJSONHandler(os.Stdout, nil)),
	}
}

// Init implements Hello.
func (g *HelloGrain) Init(ctx cluster.GrainContext) {
	g.logger.Info("HelloGrain Init")
}

// ReceiveDefault implements Hello.
func (g *HelloGrain) ReceiveDefault(ctx cluster.GrainContext) {
	g.logger.Info("HelloGrain ReceiveDefault")
}

// InvokeService implements Hello.
func (g *HelloGrain) InvokeService(req *InvokeServiceRequest, respond func(*InvokeServiceResponse), onError func(error), ctx cluster.GrainContext) error {
	if ctx.Identity() == "1" {
		client := GetHelloGrainClient(ctx.Cluster(), "2")
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
		resp, err := client.DoWork(&DoWorkRequest{Name: "Charlie"})
		if err != nil {
			return err
		}

		respond(&InvokeServiceResponse{
			Message: "Hello " + req.Name + " from " + ctx.Identity() + " and " + resp.Message,
		})
	}

	return nil
}

// DoWork implements Hello.
func (g *HelloGrain) DoWork(req *DoWorkRequest, ctx cluster.GrainContext) (*DoWorkResponse, error) {
	resp := &DoWorkResponse{
		Message: "Work done",
	}

	return resp, nil
}

// Terminate implements Hello.
func (g *HelloGrain) Terminate(ctx cluster.GrainContext) {
	g.logger.Info("HelloGrain Terminate")
}
