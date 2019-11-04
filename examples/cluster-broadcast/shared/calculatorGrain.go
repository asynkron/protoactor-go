package shared

import "github.com/otherview/protoactor-go/cluster"

type CalcGrain struct {
	cluster.Grain
	total int64
}

func (c *CalcGrain) Init(id string)  {
	c.Grain.Init(id)
	c.total = 0

	// register with the tracker
	trackerGrain := GetTrackerGrain("singleTrackerGrain")
	trackerGrain.RegisterGrain(&RegisterMessage{GrainId: c.ID()})
}

func (c *CalcGrain) Terminate()  {

	// deregister with the tracker
	trackerGrain := GetTrackerGrain("singleTrackerGrain")
	trackerGrain.DeregisterGrain(&RegisterMessage{GrainId: c.ID()})
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