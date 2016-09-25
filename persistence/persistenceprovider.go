package persistence

type PersistenceProvider interface {
	GetSnapshotInterval() int
	GetSnapshot(actorName string) (interface{}, bool)
	GetEvents(actorName string) []PersistentMessage
	GetPersistSnapshot(actorName string) func(snapshot interface{})
	PersistEvent(actorName string, event PersistentMessage)
}
