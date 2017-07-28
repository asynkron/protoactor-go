package persistence

import (
	"sync"

	"github.com/golang/protobuf/proto"
)

type snapshotEntry struct {
	eventIndex int
	snapshot   proto.Message
}

type InMemoryProvider struct {
	snapshotInterval int
	snapshots        map[string]*snapshotEntry // actorName -> a snapshot entry
	snapshotLock     *sync.Mutex
	events           map[string][]proto.Message // actorName -> a list of events
	eventsLock       *sync.Mutex
}

func NewInMemoryProvider(snapshotInterval int) *InMemoryProvider {
	return &InMemoryProvider{
		snapshotInterval: snapshotInterval,
		snapshots:        make(map[string]*snapshotEntry),
		snapshotLock:     &sync.Mutex{},
		events:           make(map[string][]proto.Message),
		eventsLock:       &sync.Mutex{},
	}
}

func (provider *InMemoryProvider) Restart() {}

func (provider *InMemoryProvider) GetSnapshotInterval() int {
	return provider.snapshotInterval
}

func (provider *InMemoryProvider) GetSnapshot(actorName string) (snapshot interface{}, eventIndex int, ok bool) {
	provider.snapshotLock.Lock()
	defer provider.snapshotLock.Unlock()
	entry, ok := provider.snapshots[actorName]
	if !ok {
		return nil, 0, false
	}
	return entry.snapshot, entry.eventIndex, true
}

func (provider *InMemoryProvider) PersistSnapshot(actorName string, eventIndex int, snapshot proto.Message) {
	provider.snapshotLock.Lock()
	defer provider.snapshotLock.Unlock()
	provider.snapshots[actorName] = &snapshotEntry{eventIndex: eventIndex, snapshot: snapshot}
}

func (provider *InMemoryProvider) GetEvents(actorName string, eventIndexStart int, callback func(e interface{})) {
	provider.eventsLock.Lock()
	defer provider.eventsLock.Unlock()
	for _, e := range provider.events[actorName][eventIndexStart:] {
		callback(e)
	}
}

func (provider *InMemoryProvider) PersistEvent(actorName string, eventIndex int, event proto.Message) {
	provider.eventsLock.Lock()
	defer provider.eventsLock.Unlock()
	provider.events[actorName] = append(provider.events[actorName], event)
}
