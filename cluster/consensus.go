// Copyright (C) 2015-2022 Asynkron AB All rights reserved

package cluster

import (
	"context"
	"sync"

	"github.com/google/uuid"
)

// this data structure is used to host consensus check results, the
// contents are protected from data races as it embeds RWMutex.
type consensusResult struct {
	sync.Mutex

	consensus bool
	value     interface{}
}

type ConsensusHandler interface {
	GetID() string
	TryGetConsensus(context.Context) (interface{}, bool)
}

type gossipConsensusHandler struct {
	ID     string
	result *consensusResult
}

func NewGossipConsensusHandler() *gossipConsensusHandler {
	handler := gossipConsensusHandler{
		ID: uuid.New().String(),
		result: &consensusResult{
			consensus: false,
			value:     nil,
		},
	}

	return &handler
}

func (hdl *gossipConsensusHandler) GetID() string { return hdl.ID }

func (hdl *gossipConsensusHandler) TryGetConsensus(context.Context) (interface{}, bool) {
	// wait until our result is available
	hdl.result.Lock()
	defer hdl.result.Unlock()

	return hdl.result.value, hdl.result.consensus
}

func (hdl *gossipConsensusHandler) TrySetConsensus(consensus interface{}) {
	hdl.result.Lock()
	go func() {
		defer hdl.result.Unlock()

		hdl.result.value = consensus
		hdl.result.consensus = true
	}()
}

func (hdl *gossipConsensusHandler) TryResetConsensus() {
	// this is a noop for now need to discuss the right
	// approach for check waiting in Go as might be another
	// way of expressing this
}
