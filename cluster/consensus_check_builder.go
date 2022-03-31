// Copyright (C) 2015-2022 Asynkron AB All rights reserved

package cluster

import (
	fmt "fmt"
	"strings"

	"github.com/asynkron/protoactor-go/log"
	"google.golang.org/protobuf/types/known/anypb"
)

type ConsensusCheckDefinition interface {
	Check() *ConsensusCheck
	AffectedKeys() map[string]struct{}
}

type consensusValue struct {
	Key   string
	Value func(*anypb.Any) interface{}
}

type consensusMemberValue struct {
	memberID string
	key      string
	value    uint64
}

type ConsensusCheckBuilder struct {
	getConsensusValues []*consensusValue
	check              ConsensusChecker
}

func NewConsensusCheckBuilder(key string, getValue func(*anypb.Any) interface{}) *ConsensusCheckBuilder {

	builder := ConsensusCheckBuilder{
		getConsensusValues: []*consensusValue{
			&consensusValue{
				Key:   key,
				Value: getValue,
			},
		},
	}
	builder.check = builder.build()
	return &builder
}

// Builds a new ConsensusHandler and ConsensusCheck values and returns pointers to them
func (ccb *ConsensusCheckBuilder) Build() (ConsensusHandler, *ConsensusCheck) {

	handle := NewGossipConsensusHandler()
	onConsensus := handle.TrySetConsensus
	lostConsensus := handle.TryResetConsensus

	check := func() *ConsensusCheck {

		hasConsensus := ccb.Check()
		hadConsensus := false

		checkConsensus := func(state *GossipState, members map[string]empty) {

			consensus, value := hasConsensus(state, members)
			if consensus {
				if hadConsensus {
					return
				}

				onConsensus(value)
				hadConsensus = true
			} else if hadConsensus {
				lostConsensus()
				hadConsensus = false
			}
		}

		consensusCheck := NewConsensusCheck(ccb.AffectedKeys(), checkConsensus)
		return &consensusCheck
	}

	return handle, check()
}

func (ccb *ConsensusCheckBuilder) Check() ConsensusChecker { return ccb.check }

func (ccb *ConsensusCheckBuilder) AffectedKeys() []string {

	keys := []string{}
	for _, value := range ccb.getConsensusValues {
		keys = append(keys, value.Key)
	}
	return keys
}

func (ccb *ConsensusCheckBuilder) MapToValue(valueTuple *consensusValue) func(string, *GossipMemberState) (string, string, uint64) {

	// REVISIT: in .NET implementation the ConsensusCheckBuilder can be of any given T type
	//          so this method returns (string, string, T) in .NET, it just feels wrong to
	//          return an interface{} from here as so far only checkers for uint64 are
	//          being used but this is not acceptable and we shall put this implementation
	//          on par with .NET version, so maybe with new go1.18 generics or making the
	//          ConsensusCheckBuilder struct to store an additional field of type empty
	//          interface to operate with internally and then provide of a custom callback
	//          from users of the data structure to convert back and forth ¯\_(ツ)_/¯
	key := valueTuple.Key
	unpack := valueTuple.Value

	return func(member string, state *GossipMemberState) (string, string, uint64) {

		var value uint64

		gossipKey, ok := state.Values[key]
		if !ok {
			value = 0
		} else {
			// REVISIT: the valueTuple is here supposedly to be able to convert
			//          the protobuf Any values contained by GossipMemberState
			//          into the right value, this is true in the .NET version
			//          as ConsensusCheckBuilder is defined as a generic type
			//          ConsensusCheckBuilder<T> so the unpacker can unpack from
			//          Any into T but we can not do that (for now) so we have
			//          to stick to unpack to the concrete uint64 type here
			value = unpack(gossipKey.Value).(uint64)
		}
		return member, key, value
	}
}

func (ccb *ConsensusCheckBuilder) build() func(*GossipState, map[string]empty) (bool, interface{}) {

	getValidMemberStates := func(state *GossipState, ids map[string]empty, result []map[string]*GossipMemberState) {

		for member, memberState := range state.Members {
			if _, ok := ids[member]; ok {
				result = append(result, map[string]*GossipMemberState{
					member: memberState,
				})
			}
		}
	}

	showLog := func(hasConsensus bool, topologyHash uint64, valueTuples []*consensusMemberValue) {

		if plog.Level() == log.DebugLevel {
			groups := map[string]int{}
			for _, memberValue := range valueTuples {
				key := fmt.Sprintf("%s:%d", memberValue.key, memberValue.value)
				if _, ok := groups[key]; ok {
					groups[key]++
				} else {
					groups[key] = 1
				}
			}

			for k, value := range groups {
				suffix := strings.Split(k, ":")[0]
				if value > 1 {
					suffix = fmt.Sprintf("%s, %d nodes", k, value)
				}
				plog.Debug("consensus", log.Bool("consensus", hasConsensus), log.String("values", suffix))
			}
		}
	}

	if len(ccb.getConsensusValues) == 1 {
		mapToValue := ccb.MapToValue(ccb.getConsensusValues[0])

		return func(state *GossipState, ids map[string]empty) (bool, interface{}) {

			memberStates := []map[string]*GossipMemberState{}
			getValidMemberStates(state, ids, memberStates)

			if len(memberStates) < len(ids) { // Not all members have state...
				return false, nil
			}

			valueTuples := []*consensusMemberValue{}
			for _, memberState := range memberStates {
				for id, state := range memberState {
					member, key, value := mapToValue(id, state)
					valueTuples = append(valueTuples, &consensusMemberValue{member, key, value})
				}
			}

			hasConsensus, topologyHash := ccb.HasConsensus(valueTuples)
			showLog(hasConsensus, topologyHash, valueTuples)

			return hasConsensus, topologyHash
		}
	}

	return func(state *GossipState, ids map[string]empty) (bool, interface{}) {

		memberStates := []map[string]*GossipMemberState{}
		getValidMemberStates(state, ids, memberStates)

		if len(memberStates) < len(ids) { // Not all members have state...
			return false, nil
		}

		valueTuples := []*consensusMemberValue{}
		for _, consensusValues := range ccb.getConsensusValues {
			mapToValue := ccb.MapToValue(consensusValues)
			for _, memberState := range memberStates {
				for id, state := range memberState {
					member, key, value := mapToValue(id, state)
					valueTuples = append(valueTuples, &consensusMemberValue{member, key, value})
				}
			}
		}

		hasConsensus, topologyHash := ccb.HasConsensus(valueTuples)
		showLog(hasConsensus, topologyHash, valueTuples)

		return hasConsensus, topologyHash
	}
}

func (ccb *ConsensusCheckBuilder) HasConsensus(memberValues []*consensusMemberValue) (bool, uint64) {

	var hasConsensus bool
	var topologyHash uint64

	if len(memberValues) == 0 {
		return hasConsensus, topologyHash
	}

	first := memberValues[0]
	for i, next := range memberValues {
		if i == 0 {
			continue
		}

		if first.value != next.value {
			return hasConsensus, topologyHash
		}
	}

	hasConsensus = true
	topologyHash = first.value
	return hasConsensus, topologyHash
}
