package persistence

type Replay struct{}
type ReplayComplete struct{}
type OfferSnapshot struct {
	Snapshot interface{}
}
type RequestSnapshot struct{}
