package persistence

type (
	Replay         struct{}
	ReplayComplete struct{}
	OfferSnapshot  struct {
		Snapshot any
	}
)
type RequestSnapshot struct{}
