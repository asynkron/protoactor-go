package persistence

import "github.com/golang/protobuf/proto"

type snapshotEntry struct {
	eventIndex int
	snapshot   proto.Message
}

type InMemoryProvider struct {
	snapshotInterval int
	snapshots        map[string]*snapshotEntry  // actorName -> a snapshot entry
	events           map[string][]proto.Message // actorName -> a list of events
}

func NewInMemoryProvider(snapshotInterval int) *InMemoryProvider {
	return &InMemoryProvider{
		snapshotInterval: snapshotInterval,
		snapshots:        make(map[string]*snapshotEntry),
		events:           make(map[string][]proto.Message),
	}
}

func (provider *InMemoryProvider) Restart() {}

func (provider *InMemoryProvider) GetSnapshotInterval() int {
	return provider.snapshotInterval
}

func (provider *InMemoryProvider) GetSnapshot(actorName string) (snapshot interface{}, eventIndex int, ok bool) {
	entry, ok := provider.snapshots[actorName]
	if !ok {
		return nil, 0, false
	}
	return entry.snapshot, entry.eventIndex, true
}

func (provider *InMemoryProvider) PersistSnapshot(actorName string, eventIndex int, snapshot proto.Message) {
	provider.snapshots[actorName] = &snapshotEntry{eventIndex: eventIndex, snapshot: snapshot}
}

func (provider *InMemoryProvider) GetEvents(actorName string, eventIndexStart int, callback func(e interface{})) {
	for _, e := range provider.events[actorName][eventIndexStart:] {
		callback(e)
	}
}

func (provider *InMemoryProvider) PersistEvent(actorName string, eventIndex int, event proto.Message) {
	provider.events[actorName] = append(provider.events[actorName], event)
}
