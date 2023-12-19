// Copyright (C) 2015-2022 Asynkron AB All rights reserved

package cluster

import (
	"log/slog"
	"time"

	"github.com/asynkron/gofun/set"
	"github.com/asynkron/protoactor-go/actor"
)

// convenience customary type to represent an empty value
// that takes no space in memory.
type empty struct{}

// Actor used to send gossip messages around
type GossipActor struct {
	gossipRequestTimeout time.Duration
	gossip               Gossip

	/// Message throttler
	throttler actor.ShouldThrottle
}

// Creates a new GossipActor and returns a pointer to its location in the heap
func NewGossipActor(requestTimeout time.Duration, myID string, getBlockedMembers func() set.Set[string], fanOut int, maxSend int, system *actor.ActorSystem) *GossipActor {

	logger := system.Logger()
	informer := newInformer(myID, getBlockedMembers, fanOut, maxSend, logger)
	gossipActor := GossipActor{
		gossipRequestTimeout: requestTimeout,
		gossip:               informer,
	}

	gossipActor.throttler = actor.NewThrottleWithLogger(logger, 3, 60*time.Second, func(logger *slog.Logger, counter int32) {
		logger.Debug("[Gossip] Sending GossipRequest", slog.Int("throttled", int(counter)))
	})

	return &gossipActor
}

// Receive method.
func (ga *GossipActor) Receive(ctx actor.Context) {
	switch r := ctx.Message().(type) {
	case *actor.Started, *actor.Stopping, *actor.Stopped:
		// pass
	case *SetGossipStateKey:
		ga.onSetGossipStateKey(r, ctx)
	case *GetGossipStateRequest:
		ga.onGetGossipStateKey(r, ctx)
	case *GossipRequest:
		ga.onGossipRequest(r, ctx)
	case *SendGossipStateRequest:
		ga.onSendGossipState(ctx)
	case *AddConsensusCheck:
		ga.onAddConsensusCheck(r)
	case *RemoveConsensusCheck:
		ga.onRemoveConsensusCheck(r)
	case *ClusterTopology:
		ga.onClusterTopology(r)
	case *GossipResponse:
		ctx.Logger().Error("GossipResponse should not be received by GossipActor") // it should be a response to a request
	default:
		ctx.Logger().Warn("Gossip received unknown message request", slog.Any("message", r))
	}
}

func (ga *GossipActor) onClusterTopology(topology *ClusterTopology) {
	ga.gossip.UpdateClusterTopology(topology)
}

func (ga *GossipActor) onAddConsensusCheck(r *AddConsensusCheck) {
	ga.gossip.AddConsensusCheck(r.ID, r.Check)
}

func (ga *GossipActor) onRemoveConsensusCheck(r *RemoveConsensusCheck) {
	ga.gossip.RemoveConsensusCheck(r.ID)
}

func (ga *GossipActor) onGetGossipStateKey(r *GetGossipStateRequest, ctx actor.Context) {
	state := ga.gossip.GetState(r.Key)
	res := NewGetGossipStateResponse(state)
	ctx.Respond(&res)
}

func (ga *GossipActor) onGossipRequest(r *GossipRequest, ctx actor.Context) {
	if ga.throttler() == actor.Open {
		ctx.Logger().Debug("OnGossipRequest", slog.Any("sender", ctx.Sender()))
	}
	ga.ReceiveState(r.State, ctx)

	if !GetCluster(ctx.ActorSystem()).MemberList.ContainsMemberID(r.MemberId) {
		ctx.Logger().Warn("Got gossip request from unknown member", slog.String("MemberId", r.MemberId))

		// nothing to send, do not provide sender or state payload
		// ctx.Respond(&GossipResponse{State: &GossipState{Members: make(map[string]*GossipState_GossipMemberState)}})
		ctx.Respond(&GossipResponse{})

		return
	}

	memberState := ga.gossip.GetMemberStateDelta(r.MemberId)
	if !memberState.HasState {
		ctx.Logger().Warn("Got gossip request from member, but no state was found", slog.String("MemberId", r.MemberId))

		// nothing to send, do not provide sender or state payload
		ctx.Respond(&GossipResponse{})

		return
	}

	ctx.Respond(&GossipResponse{})
	return

	// turn off acking for now

	//msg := GossipResponse{
	//	State: memberState.State,
	//}
	//future := ctx.RequestFuture(ctx.Sender(), &msg, GetCluster(ctx.ActorSystem()).Config.GossipRequestTimeout)
	//
	//ctx.ReenterAfter(future, func(res interface{}, err error) {
	//	if err != nil {
	//		plog.Warn("onGossipRequest failed", log.String("MemberId", r.MemberId), log.Error(err))
	//		return
	//	}
	//
	//	if _, ok := res.(*GossipResponseAck); ok {
	//		memberState.CommitOffsets()
	//		return
	//	}
	//
	//	m, ok := res.(proto.Message)
	//	if !ok {
	//		plog.Warn("onGossipRequest failed", log.String("MemberId", r.MemberId), log.Error(err))
	//		return
	//	}
	//	n := string(proto.MessageName(m).Name())
	//
	//	plog.Error("onGossipRequest received unknown response message", log.String("type", n), log.Message(r))
	//})
}

func (ga *GossipActor) onSetGossipStateKey(r *SetGossipStateKey, ctx actor.Context) {
	key, message := r.Key, r.Value
	ga.gossip.SetState(key, message)

	if ctx.Sender() != nil {
		ctx.Respond(&SetGossipStateResponse{})
	}
}

func (ga *GossipActor) onSendGossipState(ctx actor.Context) {
	ga.gossip.SendState(func(memberState *MemberStateDelta, member *Member) {
		ga.sendGossipForMember(member, memberState, ctx)
	})
	ctx.Respond(&SendGossipStateResponse{})
}

func (ga *GossipActor) ReceiveState(remoteState *GossipState, ctx actor.Context) {
	// stream our updates
	updates := ga.gossip.ReceiveState(remoteState)
	for _, update := range updates {
		ctx.ActorSystem().EventStream.Publish(update)
	}
}

func (ga *GossipActor) sendGossipForMember(member *Member, memberStateDelta *MemberStateDelta, ctx actor.Context) {
	pid := actor.NewPID(member.Address(), DefaultGossipActorName)
	if ga.throttler() == actor.Open {
		ctx.Logger().Debug("Sending GossipRequest", slog.String("MemberId", member.Id))
	}

	// a short timeout is massively important, we cannot afford hanging around waiting
	// for timeout, blocking other gossips from getting through

	msg := GossipRequest{
		MemberId: member.Id,
		State:    memberStateDelta.State,
	}
	future := ctx.RequestFuture(pid, &msg, ga.gossipRequestTimeout)

	ctx.ReenterAfter(future, func(res interface{}, err error) {
		if err != nil {
			ctx.Logger().Warn("sendGossipForMember failed", slog.String("MemberId", member.Id), slog.Any("error", err))
			return
		}

		resp, ok := res.(*GossipResponse)
		if !ok {
			ctx.Logger().Error("sendGossipForMember received unknown response message", slog.Any("message", resp))

			return
		}

		memberStateDelta.CommitOffsets()

		if resp.State != nil {
			ga.ReceiveState(resp.State, ctx)
		}
	})
}
