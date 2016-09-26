package persistence

import (
	proto "github.com/golang/protobuf/proto"
)

//PersistentEvent is a marker interface for persistent events
type PersistentEvent interface {
	Persistent()
}

//Provider is the abstraction used for persistence
type Provider interface {
	GetSnapshotInterval() int
	GetSnapshot(actorName string) (interface{}, bool)
	GetEvents(actorName string) []proto.Message
	GetPersistSnapshot(actorName string) func(snapshot interface{})
	PersistEvent(actorName string, eventIndex int, event proto.Message)
}
