// Copyright (C) 2015-2022 Asynkton AB All rights reserved

package cluster

import (
	"google.golang.org/protobuf/proto"
)

// customary type that defines a states sender callback.
type LocalStateSender func(memberStateDelta *MemberStateDelta, member *Member)

// This interface must be implemented by any value that.
// wants to be used as a gossip state storage
type GossipStateStorer interface {
	GetState(key string) map[string]*GossipKeyValue
	SetState(key string, value proto.Message)
}

// This interface must be implemented by any value that
// wants to add or remove consensus checkers
type GossipConsensusChecker interface {
	AddConsensusCheck(id string, check *ConsensusCheck)
	RemoveConsensusCheck(id string)
}

// This interface must be implemented by any value that
// wants to react to cluster topology events
type GossipCore interface {
	UpdateClusterTopology(topology *ClusterTopology)
	ReceiveState(remoteState *GossipState) []*GossipUpdate
	SendState(sendStateToMember LocalStateSender)
	GetMemberStateDelta(targetMemberID string) *MemberStateDelta
}

// The Gossip interface must be implemented by any value
// that pretends to participate with-in the Gossip protocol
type Gossip interface {
	GossipStateStorer
	GossipConsensusChecker
	GossipCore
}
