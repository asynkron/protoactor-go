package persistence

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
	events []PersistentMessage //fake database entries, only for a single actor
}

var InMemory *InMemoryProvider = &InMemoryProvider{}

func (provider *InMemoryProvider) GetEvents(actorName string) []PersistentMessage {
	return provider.events
}

func (provider *InMemoryProvider) PersistEvent(actorName string, event PersistentMessage) {
	provider.events = append(provider.events, event)
}
