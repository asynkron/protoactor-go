package persistence

import (
	proto "github.com/golang/protobuf/proto"
)

//Provider is the abstraction used for persistence
type Provider interface {
	GetSnapshotInterval() int
	GetSnapshot(actorName string) (interface{}, bool)
	GetEvents(actorName string, callback func(e interface{}))
	GetPersistSnapshot(actorName string) func(snapshot interface{})
	PersistEvent(actorName string, eventIndex int, event proto.Message)
}
