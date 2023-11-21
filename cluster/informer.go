// Copyright (C) 2015-2022 Asynkron AB All rights reserved

package cluster

import (
	"fmt"
	"log/slog"
	"math/rand"
	"reflect"
	"time"

	"github.com/asynkron/gofun/set"
	"github.com/asynkron/protoactor-go/actor"
	"google.golang.org/protobuf/proto"
)

const (
	TopologyKey       string = "topology"
	HearthbeatKey     string = "heathbeat"
	GracefullyLeftKey string = "left"
)

// create and seed a pseudo random numbers generator
var rnd = rand.New(rand.NewSource(time.Now().UnixMicro()))

// The Informer data structure implements the Gossip interface
type Informer struct {
	myID              string
	localSeqNumber    int64
	state             *GossipState
	committedOffsets  map[string]int64
	activeMemberIDs   map[string]empty
	otherMembers      []*Member
	consensusChecks   *ConsensusChecks
	getBlockedMembers func() set.Set[string]
	gossipFanOut      int
	gossipMaxSend     int
	throttler         actor.ShouldThrottle
	logger            *slog.Logger
}

// makes sure Informer complies with the Gossip interface
var _ Gossip = (*Informer)(nil)

// Creates a new Informer value with the given properties and returns
// back a pointer to its memory location in the heap
func newInformer(myID string, getBlockedMembers func() set.Set[string], fanOut int, maxSend int, logger *slog.Logger) *Informer {
	informer := Informer{
		myID: myID,
		state: &GossipState{
			Members: map[string]*GossipState_GossipMemberState{},
		},
		committedOffsets:  map[string]int64{},
		activeMemberIDs:   map[string]empty{},
		otherMembers:      []*Member{},
		consensusChecks:   NewConsensusChecks(),
		getBlockedMembers: getBlockedMembers,
		gossipFanOut:      fanOut,
		gossipMaxSend:     maxSend,
		logger:            logger,
	}
	informer.throttler = actor.NewThrottle(3, 60*time.Second, informer.throttledLog)
	return &informer
}

// called when there is a cluster topology update
func (inf *Informer) UpdateClusterTopology(topology *ClusterTopology) {
	var others []*Member
	for _, member := range topology.Members {
		if member.Id != inf.myID {
			others = append(others, member)
		}
	}
	inf.otherMembers = others

	active := make(map[string]empty)
	for _, member := range topology.Members {
		active[member.Id] = empty{}
	}

	inf.SetState(TopologyKey, topology)
}

// sets new update key state using the given proto message
func (inf *Informer) SetState(key string, message proto.Message) {
	inf.localSeqNumber = setKey(inf.state, key, message, inf.myID, inf.localSeqNumber)

	//if inf.throttler() == actor.Open {
	//	sequenceNumbers := map[string]uint64{}
	//
	//	for _, memberState := range inf.state.Members {
	//		for key, value := range memberState.Values {
	//			sequenceNumbers[key] = uint64(value.SequenceNumber)
	//		}
	//	}
	//
	//	// plog.Debug("Setting state", log.String("key", key), log.String("value", message.String()), log.Object("state", sequenceNumbers))
	//}

	if _, ok := inf.state.Members[inf.myID]; !ok {
		inf.logger.Error("State corrupt")
	}

	inf.checkConsensusKey(key)
}

// sends this informer local state to remote informers chosen randomly
// from the slice of other members known by this informer until gossipFanOut
// number of sent has been reached
func (inf *Informer) SendState(sendStateToMember LocalStateSender) {
	// inf.purgeBannedMembers()  // TODO
	for _, member := range inf.otherMembers {
		ensureMemberStateExists(inf.state, member.Id)
	}

	// make a copy of the otherMembers so we can sort it randomly
	otherMembers := make([]*Member, len(inf.otherMembers))
	copy(otherMembers, inf.otherMembers)

	// shuffles the order of the slice elements
	rnd.Shuffle(len(otherMembers), func(i, j int) {
		otherMembers[i], otherMembers[j] = otherMembers[j], otherMembers[i]
	})

	fanOutCount := 0
	for _, member := range otherMembers {
		memberState := inf.GetMemberStateDelta(member.Id)
		if !memberState.HasState {
			// nothing has change, skip it
			continue
		}

		// fire and forget, we handle results in ReenterAfter
		sendStateToMember(memberState, member)
		fanOutCount++

		// we reached our limit, break
		if fanOutCount >= inf.gossipFanOut {
			break
		}
	}
}

func (inf *Informer) GetMemberStateDelta(targetMemberID string) *MemberStateDelta {
	var count int

	// newState will old the final new state to be sent
	newState := GossipState{Members: make(map[string]*GossipState_GossipMemberState)}

	// hashmaps in Go are random by nature so no need to randomize state.Members
	pendingOffsets := inf.committedOffsets

	// create a new map with gossipMaxSend entries max
	members := make(map[string]*GossipState_GossipMemberState)

	// add ourselves to the gossip list if we are in the members state
	if member, ok := inf.state.Members[inf.myID]; ok {
		members[inf.myID] = member
		count++
	}

	// Go hash maps are unordered by nature so we don't need to randomize them
	// iterate over our state members skipping ourselves and add them to the
	// local `newState` variable until gossipMaxSend is reached
	for id, member := range inf.state.Members {
		if id == inf.myID {
			continue
		}

		count++
		members[id] = member

		if count > inf.gossipMaxSend {
			break
		}
	}

	// now we iterate over our subset of members and proceed to send them if applicable
	for memberID, memberState := range members {

		// create an empty state
		newMemberState := GossipState_GossipMemberState{
			Values: make(map[string]*GossipKeyValue),
		}

		watermarkKey := fmt.Sprintf("%s.%s", targetMemberID, memberID)

		// get the water mark
		watermark := inf.committedOffsets[watermarkKey]
		newWatermark := watermark

		// for each value in member state
		for key, value := range memberState.Values {

			if value.SequenceNumber <= watermark {
				continue
			}

			if value.SequenceNumber > newWatermark {
				newWatermark = value.SequenceNumber
			}

			newMemberState.Values[key] = value
		}

		// do not send memberStates that we have no new data for
		if len(newMemberState.Values) > 0 {
			newState.Members[memberID] = &newMemberState
			pendingOffsets[watermarkKey] = newWatermark
		}
	}

	hasState := reflect.DeepEqual(inf.committedOffsets, pendingOffsets)
	memberState := &MemberStateDelta{
		TargetMemberID: targetMemberID,
		HasState:       hasState,
		State:          &newState,
		CommitOffsets: func() {
			inf.commitPendingOffsets(pendingOffsets)
		},
	}

	return memberState
}

// adds a new consensus checker to this informer
func (inf *Informer) AddConsensusCheck(id string, check *ConsensusCheck) {
	inf.consensusChecks.Add(id, check)

	// check when adding, if we are already consistent
	check.check(inf.state, inf.activeMemberIDs)
}

// removes a consensus checker from this informer
func (inf *Informer) RemoveConsensusCheck(id string) {
	inf.consensusChecks.Remove(id)
}

// retrieves this informer current state for the given key
// returns map containing each known member id and their value
func (inf *Informer) GetState(key string) map[string]*GossipKeyValue {
	entries := make(map[string]*GossipKeyValue)

	for memberID, memberState := range inf.state.Members {
		if value, ok := memberState.Values[key]; ok {
			entries[memberID] = value
		}
	}

	return entries
}

// receives a remote informer state
func (inf *Informer) ReceiveState(remoteState *GossipState) []*GossipUpdate {
	updates, newState, updatedKeys := mergeState(inf.state, remoteState)
	if len(updates) == 0 {
		return nil
	}

	inf.state = newState
	keys := make([]string, 0, len(updatedKeys))
	for k := range updatedKeys {
		keys = append(keys, k)
	}

	inf.CheckConsensus(keys...)
	return updates
}

// check consensus for the given keys
func (inf *Informer) CheckConsensus(updatedKeys ...string) {
	for _, consensusCheck := range inf.consensusChecks.GetByUpdatedKeys(updatedKeys) {
		consensusCheck.check(inf.state, inf.activeMemberIDs)
	}
}

// runs checkers on key updates
func (inf *Informer) checkConsensusKey(updatedKey string) {
	for _, consensusCheck := range inf.consensusChecks.GetByUpdatedKey(updatedKey) {
		consensusCheck.check(inf.state, inf.activeMemberIDs)
	}
}

func (inf *Informer) commitPendingOffsets(offsets map[string]int64) {
	for key, seqNumber := range offsets {
		if offset, ok := inf.committedOffsets[key]; !ok || offset < seqNumber {
			inf.committedOffsets[key] = seqNumber
		}
	}
}

func (inf *Informer) throttledLog(counter int32) {
	inf.logger.Debug("[Gossip] Setting State", slog.Int("throttled", int(counter)))
}
