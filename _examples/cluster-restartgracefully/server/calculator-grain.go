package main

import (
	"cluster-restartgracefully/cache"
	"cluster-restartgracefully/shared"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/log"
)

type CalcGrain struct {
	total int64
}

func (c *CalcGrain) Init(ci *cluster.ClusterIdentity, cl *cluster.Cluster) {
	c.Grain.Init(ci, cl)
	c.total = cache.GetCountor(c.Identity())
	plog.Info("start", log.String("id", c.Identity()), log.Int64("number", c.total))
}

func (c *CalcGrain) Terminate() {
	id := c.Grain.Identity()
	cache.SetCountor(id, c.total)
	plog.Info("stop", log.String("id", id), log.Int64("number", c.total))
}

func (c *CalcGrain) ReceiveDefault(ctx actor.Context) {
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
