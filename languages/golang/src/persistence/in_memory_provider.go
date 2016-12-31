package persistence

import "github.com/gogo/protobuf/proto"

type NoSnapshotSupport struct {
}

func (provider *NoSnapshotSupport) GetSnapshotInterval() int {
	return 0 //snapshotting is disabled
}

func (provider *NoSnapshotSupport) GetPersistSnapshot(actorName string) func(snapshot interface{}) {
	return nil
}

func (provider *NoSnapshotSupport) GetSnapshot(actorName string) (interface{}, bool) {
	return nil, false
}

type InMemoryProvider struct {
	*NoSnapshotSupport
	events []proto.Message //fake database entries, only for a single actor
}

var InMemory *InMemoryProvider = &InMemoryProvider{}

func (provider *InMemoryProvider) GetEvents(actorName string, callback func(event interface{})) {
	for _, e := range provider.events {
		callback(e)
	}
}

func (provider *InMemoryProvider) PersistEvent(actorName string, eventIndex int, event proto.Message) {
	provider.events = append(provider.events, event)
}
