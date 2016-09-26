package persistence

import "github.com/gogo/protobuf/proto"

type PersistentEvent interface {
	Persistent()
}

type PersistenceProvider interface {
	GetSnapshotInterval() int
	GetSnapshot(actorName string) (interface{}, bool)
	GetEvents(actorName string) []proto.Message
	GetPersistSnapshot(actorName string) func(snapshot interface{})
	PersistEvent(actorName string, eventIndex int, event proto.Message)
}
