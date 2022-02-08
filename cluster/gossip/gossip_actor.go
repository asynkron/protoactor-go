// Copyright (C) 2015-2022 Asynkron AB All rights reserved

package gossip

import (
	"time"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/cluster"
	"github.com/AsynkronIT/protoactor-go/log"
)

// convenience customary type to represent an empty value
// that takes no space in memory
type empty struct{}

// Actor used to send gossip messages around
type GossipActor struct {
	gossipRequestTimeout time.Duration
	gossip               Gossip
}

// Creates a new GossipActor and returns a pointer to its location in the heap
func NewGossipActor(requestTimeout time.Duration, myID string, getBlockedMembers func() map[string]empty, fanOut int, maxSend int) *GossipActor {

	gossipActor := GossipActor{
		gossipRequestTimeout: requestTimeout,
		gossip:               newInformer(myID, getBlockedMembers, fanOut, maxSend),
	}
	return &gossipActor
}

// Receive method
func (ga *GossipActor) Receive(ctx actor.Context) {

	switch r := ctx.Message().(type) {
	case *SetGossipStateKey:
		ga.onSetGossipStateKey(r, ctx)
	case *GetGossipStateRequest:
		ga.onGetGossipStateKey(r, ctx)
	case *GossipRequest:
		ga.onGossipRequest(r, ctx)
	case *SendGossipStateRequest:
		ga.onSendGossipState(ctx)
	case *AddConsensusCheck:
		ga.onAddConsensusCheck(r, ctx)
	case *RemoveConsensusCheck:
		ga.onRemoveConsensusCheck(r, ctx)
	case *ClusterTopology:
		ga.onClusterTopology(r)
	default:
		plog.Warn("Gossip received unknown message request", log.Message(r))
	}
}

func (ga *GossipActor) onClusterTopology(topology *ClusterTopology) {
	ga.gossip.UpdateClusterTopology(topology)
}

func (ga *GossipActor) onAddConsensusCheck(r *AddConsensusCheck, ctx actor.Context) {
	ga.gossip.AddConsensusCheck(r.ID, r.Check)
}

func (ga *GossipActor) onRemoveConsensusCheck(r *RemoveConsensusCheck, ctx actor.Context) {
	ga.gossip.RemoveConsensusCheck(r.ID)
}

func (ga *GossipActor) onGetGossipStateKey(r *GetGossipStateRequest, ctx actor.Context) {

	state := ga.gossip.GetState(r.Key)
	res := NewGetGossipStateResponse(state)
	ctx.Respond(&res)
}

func (ga *GossipActor) onGossipRequest(r *GossipRequest, ctx actor.Context) {

	plog.Debug("Gossip request", log.PID("sender", ctx.Sender()))
	ga.ReceiveState(r.State, ctx)

	if !cluster.GetCluster(ctx.ActorSystem()).MemberList.ContainsMemberID(r.MemberId) {
		plog.Warn("Got gossip request from unknown member", log.String("MemberId", r.MemberId))

		// nothing to send, do not provide sender or state payload
		// ctx.Respond(&GossipResponse{State: &GossipState{Members: make(map[string]*GossipState_GossipMemberState)}})
		ctx.Respond(&GossipResponse{})
		return
	}

	memberState := ga.gossip.GetMemberStateDelta(r.MemberId)
	if !memberState.HasState {
		plog.Warn("Got gossip request from member, but no state was found", log.String("MemberId", r.MemberId))

		// nothing to send, do not provide sender or state payload
		ctx.Respond(&GossipResponse{})
		return
	}

	msg := GossipResponse{
		State: memberState.State,
	}
	future := ctx.RequestFuture(ctx.Sender(), &msg, cluster.GetCluster(ctx.ActorSystem()).Config.GossipRequestTimeout)

	// wait until we get a response or an error from the future
	resp, err := future.Result()
	if err != nil {
		plog.Error("onSendGossipState failed", log.Error(err))
		return
	}

	if _, ok := resp.(*GossipResponseAck); ok {
		memberState.CommitOffsets()
		return
	}

	plog.Error("onSendGossipState received unknown response message", log.Message(r))
}

func (ga *GossipActor) onSetGossipStateKey(r *SetGossipStateKey, ctx actor.Context) {

	key, message := r.Key, r.Value
	ga.gossip.SetState(key, message)
	ctx.Respond(&SetGossipStateResponse{})
}

func (ga *GossipActor) onSendGossipState(ctx actor.Context) {

	ga.gossip.SendState(func(memberState *MemberStateDelta, member *cluster.Member) {
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

func (ga *GossipActor) sendGossipForMember(member *cluster.Member, memberStateDelta *MemberStateDelta, ctx actor.Context) {

	pid := actor.NewPID(member.Address(), DefaultGossipActorName)
	plog.Info("Sending GossipRequest", log.String("MemberId", member.Id))

	// a short timeout is massively important, we cannot afford hanging around waiting
	// for timeout, blocking other gossips from getting through

	msg := GossipRequest{
		MemberId: ctx.ActorSystem().ID,
		State:    memberStateDelta.State,
	}
	future := ctx.RequestFuture(pid, &msg, ga.gossipRequestTimeout)

	// wait until we get a response or an error from the future
	r, err := future.Result()
	if err != nil {
		plog.Error("onSendGossipState failed", log.Error(err))
		return
	}

	resp, ok := r.(*GossipResponse)
	if !ok {
		plog.Error("onSendGossipState received unknown response message", log.Message(r))
		return
	}

	memberStateDelta.CommitOffsets()
	if resp.State != nil {
		ga.ReceiveState(resp.State, ctx)
		if ctx.Sender() != nil {
			ctx.Send(ctx.Sender(), &GossipResponseAck{})
		}
	}
}
