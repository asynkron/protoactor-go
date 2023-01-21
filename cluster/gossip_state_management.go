package cluster

import (
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// convenience type alias
type GossipMemberState = GossipState_GossipMemberState

func ensureEntryExists(memberState *GossipMemberState, key string) *GossipKeyValue {
	value, ok := memberState.Values[key]
	if ok {
		return value
	}

	value = &GossipKeyValue{}
	memberState.Values[key] = value

	return value
}

// returns back the GossipMemberState registered in the given GossipState
// under the given memberID key, if the key doesn't exists yet it is created
func ensureMemberStateExists(state *GossipState, memberID string) *GossipMemberState {
	memberState, ok := state.Members[memberID]
	if ok {
		return memberState
	}

	memberState = &GossipMemberState{Values: make(map[string]*GossipKeyValue)}
	state.Members[memberID] = memberState

	return memberState
}

// sets the given key with the given value in the given gossip state and returns sequenceNo + 1
func setKey(state *GossipState, key string, value proto.Message, memberID string, sequenceNo int64) int64 {
	// if entry does not exists, add it
	memberState := ensureMemberStateExists(state, memberID)
	entry := ensureEntryExists(memberState, key)
	entry.LocalTimestampUnixMilliseconds = time.Now().UnixMilli()

	sequenceNo++
	entry.SequenceNumber = sequenceNo

	a, _ := anypb.New(value)
	entry.Value = a

	return sequenceNo
}

// merges the local and the incoming remote states into a new states slice and return it
func mergeState(localState *GossipState, remoteState *GossipState) ([]*GossipUpdate, *GossipState, map[string]empty) {
	// make a copy of the localState (we do not want to modify localState just yet)
	mergedState := &GossipState{Members: make(map[string]*GossipState_GossipMemberState)}
	for id, member := range localState.Members {
		mergedState.Members[id] = member
	}

	var updates []*GossipUpdate
	updatedKeys := make(map[string]empty)

	for memberID, remoteMemberState := range remoteState.Members {
		if _, ok := mergedState.Members[memberID]; !ok {
			mergedState.Members[memberID] = remoteMemberState
			for key, entry := range remoteMemberState.Values {
				update := GossipUpdate{
					MemberID:  memberID,
					Key:       key,
					Value:     entry.Value,
					SeqNumber: entry.SequenceNumber,
				}
				updates = append(updates, &update)
				entry.LocalTimestampUnixMilliseconds = time.Now().UnixMilli()
				updatedKeys[key] = empty{}
			}
			continue
		}

		// this entry exists in both mergedState and remoteState, we should merge them
		newMemberState := mergedState.Members[memberID]
		for key, remoteValue := range remoteMemberState.Values {
			// this entry does not exist in newMemberState, just copy all of it
			if _, ok := newMemberState.Values[key]; !ok {
				newMemberState.Values[key] = remoteValue
				update := GossipUpdate{
					MemberID:  memberID,
					Key:       key,
					Value:     remoteValue.Value,
					SeqNumber: remoteValue.SequenceNumber,
				}
				updates = append(updates, &update)
				remoteValue.LocalTimestampUnixMilliseconds = time.Now().UnixMilli()
				updatedKeys[key] = empty{}
				continue
			}

			newValue := newMemberState.Values[key]

			// remote value is older, ignore
			if remoteValue.SequenceNumber <= newValue.SequenceNumber {
				continue
			}

			// just replace the existing value
			newMemberState.Values[key] = remoteValue
			update := GossipUpdate{
				MemberID:  memberID,
				Key:       key,
				Value:     remoteValue.Value,
				SeqNumber: remoteValue.SequenceNumber,
			}
			updates = append(updates, &update)
			remoteValue.LocalTimestampUnixMilliseconds = time.Now().UnixMilli()
			updatedKeys[key] = empty{}
		}
	}
	return updates, mergedState, updatedKeys
}
