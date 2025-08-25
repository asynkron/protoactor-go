// Copyright (C) 2017 - 2024 Asynkron AB All rights reserved

package cluster

import (
	"context"
	"fmt"
	"log/slog"

	"google.golang.org/protobuf/types/known/anypb"
)

// ConsensusCheckDefinition produces a consensus check along with the keys it touches.
// It is implemented by ConsensusCheckBuilder.
type ConsensusCheckDefinition interface {
	Check() *ConsensusCheck
	AffectedKeys() []string
}

// ConsensusCheckBuilder aggregates value extractors to create a consensus check.
// T must be comparable so values can be checked for equality without reflection.
type ConsensusCheckBuilder[T comparable] struct {
	keys       []string
	extractors []func(*anypb.Any) (T, error)
	check      ConsensusChecker
	logger     *slog.Logger
}

// NewConsensusCheckBuilder returns a builder seeded with a single consensus value extractor.
func NewConsensusCheckBuilder[T comparable](logger *slog.Logger, key string, getValue func(*anypb.Any) (T, error)) *ConsensusCheckBuilder[T] {
	builder := &ConsensusCheckBuilder[T]{
		keys:       []string{key},
		extractors: []func(*anypb.Any) (T, error){getValue},
		logger:     logger,
	}
	builder.check = builder.build()
	return builder
}

// Build creates a new ConsensusHandler and ConsensusCheck and returns pointers to them.
func (ccb *ConsensusCheckBuilder[T]) Build() (ConsensusHandler, *ConsensusCheck) {
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

// Check returns the consensus checker produced by this builder.
func (ccb *ConsensusCheckBuilder[T]) Check() ConsensusChecker { return ccb.check }

// AffectedKeys returns the gossip keys inspected by this builder.
func (ccb *ConsensusCheckBuilder[T]) AffectedKeys() []string {
	return append([]string(nil), ccb.keys...)
}

// build constructs the ConsensusChecker used to evaluate consensus.
func (ccb *ConsensusCheckBuilder[T]) build() ConsensusChecker {
	showLog := func(hasConsensus bool, values []T) {
		if !ccb.logger.Enabled(context.TODO(), slog.LevelDebug) {
			return
		}
		groups := map[string]int{}
		for _, v := range values {
			groups[fmt.Sprintf("%v", v)]++
		}
		for k, v := range groups {
			suffix := k
			if v > 1 {
				suffix = fmt.Sprintf("%s, %d nodes", k, v)
			}
			ccb.logger.Debug("consensus", slog.Bool("consensus", hasConsensus), slog.String("values", suffix))
		}
	}

	return func(state *GossipState, ids map[string]empty) (bool, interface{}) {
		var values []T
		for id := range ids {
			memberState, ok := state.Members[id]
			if !ok {
				return false, nil
			}
			for i, key := range ccb.keys {
				gossipKey, ok := memberState.Values[key]
				if !ok {
					return false, nil
				}
				v, err := ccb.extractors[i](gossipKey.Value)
				if err != nil {
					return false, nil
				}
				values = append(values, v)
			}
		}

		if len(values) < len(ids)*len(ccb.keys) {
			return false, nil
		}

		has, val := ccb.HasConsensus(values)
		showLog(has, values)

		if has {
			return true, any(val)
		}
		return false, nil
	}
}

// HasConsensus checks whether all values in the slice are identical.
func (ccb *ConsensusCheckBuilder[T]) HasConsensus(values []T) (bool, T) {
	var zero T
	if len(values) == 0 {
		return false, zero
	}
	first := values[0]
	for _, v := range values[1:] {
		if v != first {
			return false, zero
		}
	}
	return true, first
}
