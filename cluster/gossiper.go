// Copyright (C) 2017 - 2024 Asynkron AB All rights reserved

package cluster

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/asynkron/protoactor-go/remote"

	"github.com/asynkron/gofun/set"
	"google.golang.org/protobuf/proto"

	"github.com/asynkron/protoactor-go/actor"
	"google.golang.org/protobuf/types/known/anypb"
)

// DefaultGossipActorName is the default name used for the gossip actor.
const DefaultGossipActorName string = "gossip"

// GossipUpdate Used to update gossip data when a ClusterTopology event occurs
type GossipUpdate struct {
	MemberID, Key string
	Value         *anypb.Any
	SeqNumber     int64
}

// ConsensusChecker Customary type used to provide consensus check callbacks of any type
// note: this is equivalent to (for future go v1.18):
//
//	type ConsensusChecker[T] func(GossipState, map[string]empty) (bool, T)
type ConsensusChecker func(*GossipState, map[string]empty) (bool, interface{})

// Gossiper manages gossip data and dissemination within the cluster.
type Gossiper struct {
	// The Gossiper Actor Name, defaults to "gossip"
	GossipActorName string

	// The Gossiper Cluster
	cluster *Cluster

	// The actor PID
	pid *actor.PID

	// Channel use to stop the gossip loop
	close chan struct{}

	// Message throttler
	throttler actor.ShouldThrottle
}

// Creates a new Gossiper value and return it back
func newGossiper(cl *Cluster, opts ...Option) (*Gossiper, error) {
	// create a new Gossiper value
	gossiper := &Gossiper{
		GossipActorName: DefaultGossipActorName,
		cluster:         cl,
		close:           make(chan struct{}),
	}

	// apply any given options
	for _, opt := range opts {
		opt(gossiper)
	}

	return gossiper, nil
}

// GetState retrieves gossip state for the given key.
func (g *Gossiper) GetState(key string) (map[string]*GossipKeyValue, error) {
	if g.throttler() == actor.Open {
		g.cluster.Logger().Debug("Gossiper getting state", slog.String("key", key), slog.String("gossipPid", g.pid.String()))
	}

	msg := NewGetGossipStateRequest(key)
	timeout := g.cluster.Config.TimeoutTime
	r, err := g.cluster.ActorSystem.Root.RequestFuture(g.pid, &msg, timeout).Result()
	if err != nil {
		switch err {
		case actor.ErrTimeout:
			g.cluster.Logger().Error("Could not get a response from GossipActor: request timeout", slog.Any("error", err), slog.String("gossipPid", g.pid.String()))
			return nil, err
		case actor.ErrDeadLetter:
			g.cluster.Logger().Error("remote no longer exists", slog.Any("error", err), slog.String("gossipPid", g.pid.String()))
			return nil, err
		default:
			g.cluster.Logger().Error("Could not get a response from GossipActor", slog.Any("error", err), slog.String("gossipPid", g.pid.String()))
			return nil, err
		}
	}

	// try to cast the response to GetGossipStateResponse concrete value
	response, ok := r.(*GetGossipStateResponse)
	if !ok {
		err := fmt.Errorf("could not promote %T interface to GetGossipStateResponse", r)
		g.cluster.Logger().Error("Could not get a response from GossipActor", slog.Any("error", err), slog.String("gossipPid", g.pid.String()))
		return nil, err
	}

	return response.State, nil
}

// SetState sends a fire-and-forget message to update member state.
func (g *Gossiper) SetState(gossipStateKey string, value proto.Message) {
	if g.throttler() == actor.Open {
		g.cluster.Logger().Info("Gossiper setting state", slog.String("gossipStateKey", gossipStateKey), slog.String("gossipPid", g.pid.String()))
	}

	if g.pid == nil {
		return
	}

	msg := NewGossipStateKey(gossipStateKey, value)
	g.cluster.ActorSystem.Root.Send(g.pid, &msg)
}

// SetMapState updates the value for a key within a map state.
func (g *Gossiper) SetMapState(gossipStateKey string, mapKey string, value proto.Message) {
	if g.throttler() == actor.Open {
		g.cluster.Logger().Info("Gossiper setting map state", slog.String("gossipStateKey", gossipStateKey), slog.String("gossipPid", g.pid.String()))
	}

	if g.pid == nil {
		return
	}

	msg := SetGossipMapState{
		GossipStateKey: gossipStateKey,
		MapKey:         mapKey,
		Value:          value,
	}

	g.cluster.ActorSystem.Root.Send(g.pid, &msg)
}

// GetMapState retrieves a value from a map in the gossip state.
func (g *Gossiper) GetMapState(gossipStateKey string, mapKey string) *anypb.Any {
	if g.throttler() == actor.Open {
		g.cluster.Logger().Info("Gossiper setting map state", slog.String("gossipStateKey", gossipStateKey), slog.String("gossipPid", g.pid.String()))
	}

	msg := GetGossipMapStateRequest{
		GossipStateKey: gossipStateKey,
		MapKey:         mapKey,
	}

	x, err := g.cluster.ActorSystem.Root.RequestFuture(g.pid, &msg, g.cluster.Config.TimeoutTime).Result()
	if err != nil {
		g.cluster.Logger().Error("Could not get a response from GossipActor", slog.Any("error", err), slog.String("gossipPid", g.pid.String()))
		return nil
	}
	//cast x to GetGossipMapStateResponse
	response, ok := x.(*GetGossipMapStateResponse)
	if !ok {
		err := fmt.Errorf("could not promote %T interface to GetGossipMapStateResponse", x)
		g.cluster.Logger().Error("Could not get a response from GossipActor", slog.Any("error", err), slog.String("gossipPid", g.pid.String()))
		return nil
	}
	return response.Value
}

// RemoveMapState deletes a key from a gossip map state.
func (g *Gossiper) RemoveMapState(gossipStateKey string, mapKey string) {
	if g.throttler() == actor.Open {
		g.cluster.Logger().Info("Gossiper setting map state", slog.String("gossipStateKey", gossipStateKey), slog.String("gossipPid", g.pid.String()))
	}

	if g.pid == nil {
		return
	}

	msg := RemoveGossipMapState{
		GossipStateKey: gossipStateKey,
		MapKey:         mapKey,
	}

	g.cluster.ActorSystem.Root.Send(g.pid, &msg)
}

// GetMapKeys returns all keys stored in a map gossip state.
func (g *Gossiper) GetMapKeys(gossipStateKey string) []string {
	if g.throttler() == actor.Open {
		g.cluster.Logger().Info("Gossiper setting map state", slog.String("gossipStateKey", gossipStateKey), slog.String("gossipPid", g.pid.String()))
	}

	if g.pid == nil {
		return []string{}
	}

	msg := GetGossipMapKeysRequest{
		GossipStateKey: gossipStateKey,
	}

	res, err := g.cluster.ActorSystem.Root.RequestFuture(g.pid, &msg, g.cluster.Config.TimeoutTime).Result()

	if err != nil {
		g.cluster.Logger().Error("Could not get a response from GossipActor", slog.Any("error", err), slog.String("gossipPid", g.pid.String()))
		return []string{}
	}
	//cast res to GetGossipMapKeysResponse
	response, ok := res.(*GetGossipMapKeysResponse)
	if !ok {
		err := fmt.Errorf("could not promote %T interface to GetGossipMapKeysResponse", res)
		g.cluster.Logger().Error("Could not get a response from GossipActor", slog.Any("error", err), slog.String("gossipPid", g.pid.String()))
		return []string{}
	}
	return response.MapKeys
}

// SetStateRequest sends a request that blocks to update member state.
func (g *Gossiper) SetStateRequest(key string, value proto.Message) error {
	if g.throttler() == actor.Open {
		g.cluster.Logger().Debug("Gossiper setting state", slog.String("key", key), slog.String("gossipPid", g.pid.String()))
	}

	if g.pid == nil {
		return errors.New("gossiper Actor PID is nil")
	}

	msg := NewGossipStateKey(key, value)
	r, err := g.cluster.ActorSystem.Root.RequestFuture(g.pid, &msg, g.cluster.Config.TimeoutTime).Result()
	if err != nil {
		if err == actor.ErrTimeout {
			g.cluster.Logger().Error("Could not get a response from Gossiper Actor: request timeout", slog.String("gossipPid", g.pid.String()))
			return err
		}
		g.cluster.Logger().Error("Could not get a response from Gossiper Actor", slog.Any("error", err), slog.String("gossipPid", g.pid.String()))
		return err
	}

	// try to cast the response to SetGossipStateResponse concrete value
	_, ok := r.(*SetGossipStateResponse)
	if !ok {
		err := fmt.Errorf("could not promote %T interface to SetGossipStateResponse", r)
		g.cluster.Logger().Error("Could not get a response from Gossip Actor", slog.Any("error", err), slog.String("gossipPid", g.pid.String()))
		return err
	}
	return nil
}

// SendState requests the gossip actor to broadcast its current state.
func (g *Gossiper) SendState() {
	if g.pid == nil {
		return
	}

	r, err := g.cluster.ActorSystem.Root.RequestFuture(g.pid, &SendGossipStateRequest{}, 5*time.Second).Result()
	if err != nil {
		g.cluster.Logger().Warn("Gossip could not send gossip request", slog.Any("PID", g.pid), slog.Any("error", err))
		return
	}

	if _, ok := r.(*SendGossipStateResponse); !ok {
		g.cluster.Logger().Error("Gossip SendState received unknown response", slog.Any("message", r))
	}
}

// RegisterConsensusCheck builds a consensus handler and checker for the given key and value extractor.
// The extractor unpacks the gossip state into a value used for comparison.
func (g *Gossiper) RegisterConsensusCheck(key string, getValue func(*anypb.Any) (uint64, error)) ConsensusHandler {
	definition := NewConsensusCheckBuilder(g.cluster.Logger(), key, getValue)
	consensusHandle, check := definition.Build()
	request := NewAddConsensusCheck(consensusHandle.GetID(), check)
	g.cluster.ActorSystem.Root.Send(g.pid, &request)
	return consensusHandle
}

// StartGossiping spawns the gossip actor and begins gossiping.
func (g *Gossiper) StartGossiping() error {
	var err error
	g.cluster.Logger().Info("Starting gossip")
	g.pid, err = g.cluster.ActorSystem.Root.SpawnNamed(actor.PropsFromProducerWithActorSystem(func(system *actor.ActorSystem) actor.Actor {
		return NewGossipActor(
			g.cluster.Config.GossipRequestTimeout,
			g.cluster.ActorSystem.ID,
			func() set.Set[string] {
				return g.cluster.GetBlockedMembers()
			},
			g.cluster.Config.GossipFanOut,
			g.cluster.Config.GossipMaxSend,
			system,
		)
	}), g.GossipActorName)
	if err != nil {
		g.cluster.Logger().Error("Failed to start gossip actor", slog.Any("error", err))
		return err
	}

	g.cluster.ActorSystem.EventStream.Subscribe(func(evt interface{}) {
		if topology, ok := evt.(*ClusterTopology); ok {
			g.cluster.ActorSystem.Root.Send(g.pid, topology)
		}
	})
	g.cluster.Logger().Info("Started Cluster Gossip")
	g.throttler = actor.NewThrottle(3, 60*time.Second, g.throttledLog)
	go g.gossipLoop()

	return nil
}

// Shutdown stops the gossip actor and terminates gossiping.
func (g *Gossiper) Shutdown() {
	if g.pid == nil {
		return
	}

	g.cluster.Logger().Info("Shutting down gossip")

	close(g.close)

	err := g.cluster.ActorSystem.Root.StopFuture(g.pid).Wait()
	if err != nil {
		g.cluster.Logger().Error("failed to stop gossip actor", slog.Any("error", err))
	}

	g.cluster.Logger().Info("Shut down gossip")
}

func (g *Gossiper) gossipLoop() {
	g.cluster.Logger().Debug("Starting gossip loop")

	// create a ticker that will tick each GossipInterval milliseconds
	// we do not use sleep as sleep puts the goroutine out of the scheduler
	// P, and we do not want our Gs to be scheduled out from the running Ms
	ticker := time.NewTicker(g.cluster.Config.GossipInterval)
breakLoop:
	for !g.cluster.ActorSystem.IsStopped() {
		select {
		case <-g.close:
			g.cluster.Logger().Debug("Stopping Gossip Loop")
			break breakLoop
		case <-ticker.C:
			g.blockExpiredHeartbeats()
			g.blockGracefullyLeft()
			g.SetState(HeartbeatKey, &MemberHeartbeat{
				ActorStatistics: &ActorStatistics{
					ActorCount: g.GetActorCount(),
				},
			})
			g.SendState()
		}
	}
}

// GetActorCount returns the number of actors for each registered kind.
func (g *Gossiper) GetActorCount() map[string]int64 {
	m := make(map[string]int64)
	clusterKinds := g.cluster.GetClusterKinds()
	for _, kindName := range clusterKinds {
		kind := g.cluster.GetClusterKind(kindName)
		m[kindName] = int64(kind.Count())
	}
	g.cluster.Logger().Debug("Actor Count", slog.Any("count", m))

	return m
}

// blockExpiredHeartbeats blocks members that have not sent a heartbeat for a long time
func (g *Gossiper) blockExpiredHeartbeats() {
	if g.cluster.Config.GossipInterval == 0 {
		return
	}
	t, err := g.GetState(HeartbeatKey)
	if err != nil {
		g.cluster.Logger().Error("Could not get heartbeat state", slog.Any("error", err))
		return
	}

	blockList := remote.GetRemote(g.cluster.ActorSystem).BlockList()

	blocked := make([]string, 0)

	for k, v := range t {
		if k != g.cluster.ActorSystem.ID &&
			!blockList.IsBlocked(k) &&
			time.Since(time.UnixMilli(v.LocalTimestampUnixMilliseconds)) > g.cluster.Config.HeartbeatExpiration {
			blocked = append(blocked, k)
		}
	}

	if len(blocked) > 0 {
		g.cluster.Logger().Info("Blocking members due to expired heartbeat", slog.String("members", strings.Join(blocked, ",")))
		blockList.Block(blocked...)
	}
}

// blockGracefullyLeft blocking members due to gracefully leaving
func (g *Gossiper) blockGracefullyLeft() {
	t, err := g.GetState(GracefullyLeftKey)
	if err != nil {
		g.cluster.Logger().Error("Could not get gracefully left members", slog.Any("error", err))
		return
	}

	blockList := remote.GetRemote(g.cluster.ActorSystem).BlockList()

	gracefullyLeft := make([]string, 0)
	for k := range t {
		if !blockList.IsBlocked(k) && k != g.cluster.ActorSystem.ID {
			gracefullyLeft = append(gracefullyLeft, k)
		}
	}
	if len(gracefullyLeft) > 0 {
		g.cluster.Logger().Info("Blocking members due to gracefully leaving", slog.String("members", strings.Join(gracefullyLeft, ",")))
		blockList.Block(gracefullyLeft...)
	}
}

func (g *Gossiper) throttledLog(counter int32) {
	g.cluster.Logger().Debug("Gossiper Setting State", slog.String("gossipPid", g.pid.String()), slog.Int("throttled", int(counter)))
}
