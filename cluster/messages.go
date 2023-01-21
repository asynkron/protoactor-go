package cluster

import (
	"google.golang.org/protobuf/proto"
)

// Used to query the GossipActor about a given key status
type GetGossipStateRequest struct {
	Key string
}

// Create a new GetGossipStateRequest value and return it back
func NewGetGossipStateRequest(key string) GetGossipStateRequest {
	request := GetGossipStateRequest{Key: key}
	return request
}

// Used by the GossipActor to send back the status value of a given key
type GetGossipStateResponse struct {
	State map[string]*GossipKeyValue
}

func NewGetGossipStateResponse(state map[string]*GossipKeyValue) GetGossipStateResponse {
	value := GetGossipStateResponse{
		State: state,
	}
	return value
}

// Used to setup Gossip Status Keys in the GossipActor
type SetGossipStateKey struct {
	Key   string
	Value proto.Message
}

// Create a new SetGossipStateKey value with the given data and return it back
func NewGossipStateKey(key string, value proto.Message) SetGossipStateKey {
	statusKey := SetGossipStateKey{
		Key:   key,
		Value: value,
	}
	return statusKey
}

type SendGossipStateRequest struct{}

type SendGossipStateResponse struct{}

// Used by the GossipActor to respond SetGossipStatus requests
type SetGossipStateResponse struct{}

type AddConsensusCheck struct {
	ID    string
	Check *ConsensusCheck
}

// Mimic .NET ReenterAfterCancellation on GossipActor
type RemoveConsensusCheck struct {
	ID string
}

func NewAddConsensusCheck(id string, check *ConsensusCheck) AddConsensusCheck {
	value := AddConsensusCheck{
		ID:    id,
		Check: check,
	}
	return value
}

func NewRemoveConsensusCheck(id string) RemoveConsensusCheck {
	value := RemoveConsensusCheck{
		ID: id,
	}
	return value
}
