package shared

import (
	"cluster-restartgracefully/cache"
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/cluster"
	"github.com/AsynkronIT/protoactor-go/log"
)

type CalcGrain struct {
	cluster.Grain
	total int64
}

func (c *CalcGrain) Init(id string) {
	c.Grain.Init(id)
	c.total = cache.GetCountor(id)
	plog.Info("start", log.String("id", id), log.Int64("number", c.total))
}

func (c *CalcGrain) Terminate() {
	id := c.Grain.ID()
	cache.SetCountor(id, c.total)
	plog.Info("stop", log.String("id", id), log.Int64("number", c.total))
}

func (c *CalcGrain) ReceiveDefault(ctx actor.Context) {
}

func (c *CalcGrain) Add(n *NumberRequest, ctx cluster.GrainContext) (*CountResponse, error) {
	c.total = c.total + n.Number
	return &CountResponse{Number: c.total}, nil
}

func (c *CalcGrain) Subtract(n *NumberRequest, ctx cluster.GrainContext) (*CountResponse, error) {
	c.total = c.total - n.Number
	return &CountResponse{Number: c.total}, nil
}

func (c *CalcGrain) GetCurrent(n *Void, ctx cluster.GrainContext) (*CountResponse, error) {
	return &CountResponse{Number: c.total}, nil
}
