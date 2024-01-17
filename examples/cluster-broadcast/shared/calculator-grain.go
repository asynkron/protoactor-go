package shared

import (
	"github.com/asynkron/protoactor-go/cluster"
)

type CalcGrain struct {
	total int64
}

func (c *CalcGrain) ReceiveDefault(ctx cluster.GrainContext) {
}

func (c *CalcGrain) Init(ctx cluster.GrainContext) {
	c.total = 0

	// register with the tracker
	trackerGrain := GetTrackerGrainClient(ctx.Cluster(), "singleTrackerGrain")
	trackerGrain.RegisterGrain(&RegisterMessage{GrainId: ctx.Identity()})
}

func (c *CalcGrain) Terminate(ctx cluster.GrainContext) {
	// deregister with the tracker
	trackerGrain := GetTrackerGrainClient(ctx.Cluster(), "singleTrackerGrain")
	trackerGrain.DeregisterGrain(&RegisterMessage{GrainId: ctx.Identity()})
}

func (c *CalcGrain) Add(n *NumberRequest, ctx cluster.GrainContext) (*CountResponse, error) {
	c.total = c.total + n.Number
	return &CountResponse{Number: c.total}, nil
}

func (c *CalcGrain) Subtract(n *NumberRequest, ctx cluster.GrainContext) (*CountResponse, error) {
	c.total = c.total - n.Number
	return &CountResponse{Number: c.total}, nil
}

func (c *CalcGrain) GetCurrent(n *Noop, ctx cluster.GrainContext) (*CountResponse, error) {
	return &CountResponse{Number: c.total}, nil
}
