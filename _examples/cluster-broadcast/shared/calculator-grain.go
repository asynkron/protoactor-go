package shared

import (
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
)

type CalcGrain struct {
	cluster.Grain
	total int64
}

func init() {
	// apply DI and setup logic
	CalculatorFactory(func() Calculator {
		return &CalcGrain{}
	})
}

func (c *CalcGrain) ReceiveDefault(ctx actor.Context) {

}

func (c *CalcGrain) Init(ci *cluster.ClusterIdentity, cl *cluster.Cluster) {
	c.Grain.Init(ci, cl)
	c.total = 0

	// register with the tracker
	trackerGrain := GetTrackerGrainClient(c.Cluster(), "singleTrackerGrain")
	trackerGrain.RegisterGrain(&RegisterMessage{GrainId: c.Identity()})
}

func (c *CalcGrain) Terminate() {

	// deregister with the tracker
	trackerGrain := GetTrackerGrainClient(c.Cluster(), "singleTrackerGrain")
	trackerGrain.DeregisterGrain(&RegisterMessage{GrainId: c.Identity()})
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
