package persistence

type (
	Replay         struct{}
	ReplayComplete struct{}
	OfferSnapshot  struct {
		Snapshot interface{}
	}
)
type RequestSnapshot struct{}
