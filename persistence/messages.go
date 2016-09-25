package persistence

type PersistentMessage interface {
	PersistentMessage()
}

type Replay struct{}
type ReplayComplete struct{}
type OfferSnapshot struct {
	Snapshot interface{}
}
type RequestSnapshot struct {
	PersistSnapshot func(snapshot interface{})
}
