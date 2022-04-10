package main

import (
	"cluster-restartgracefully/cache"
	"cluster-restartgracefully/shared"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/log"
)

type CalcGrain struct {
	total int64
}

func (c *CalcGrain) Init(ctx cluster.GrainContext) {
	c.total = cache.GetCountor(ctx.Identity())
	plog.Info("start", log.String("id", ctx.Identity()), log.Int64("number", c.total))
}

func (c *CalcGrain) Terminate(ctx cluster.GrainContext) {
	id := ctx.Identity()
	cache.SetCountor(id, c.total)
	plog.Info("stop", log.String("id", id), log.Int64("number", c.total))
}

func (c *CalcGrain) ReceiveDefault(ctx cluster.GrainContext) {
}

func (c *CalcGrain) Add(n *shared.NumberRequest, ctx cluster.GrainContext) (*shared.CountResponse, error) {
	c.total = c.total + n.Number
	return &shared.CountResponse{Number: c.total}, nil
}

func (c *CalcGrain) Subtract(n *shared.NumberRequest, ctx cluster.GrainContext) (*shared.CountResponse, error) {
	c.total = c.total - n.Number
	return &shared.CountResponse{Number: c.total}, nil
}

func (c *CalcGrain) GetCurrent(n *shared.Void, ctx cluster.GrainContext) (*shared.CountResponse, error) {
	return &shared.CountResponse{Number: c.total}, nil
}
