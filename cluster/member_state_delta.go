// Copyright (C) 2015-2022 Asynkron AB All rights reserved

package cluster

type MemberStateDelta struct {
	TargetMemberID string
	HasState       bool
	State          *GossipState
	CommitOffsets  func()
}
