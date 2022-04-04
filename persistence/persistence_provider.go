package persistence

import (
	"google.golang.org/protobuf/proto"
)

// Provider is the abstraction used for persistence
type Provider interface {
	GetState() ProviderState
}

// ProviderState is an object containing the implementation for the provider
type ProviderState interface {
	SnapshotStore
	EventStore

	Restart()
	GetSnapshotInterval() int
}

type SnapshotStore interface {
	GetSnapshot(actorName string) (snapshot interface{}, eventIndex int, ok bool)
	PersistSnapshot(actorName string, snapshotIndex int, snapshot proto.Message)
	DeleteSnapshots(actorName string, inclusiveToIndex int)
}

type EventStore interface {
	GetEvents(actorName string, eventIndexStart int, eventIndexEnd int, callback func(e interface{}))
	PersistEvent(actorName string, eventIndex int, event proto.Message)
	DeleteEvents(actorName string, inclusiveToIndex int)
}
