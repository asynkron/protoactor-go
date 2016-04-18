package actor

type Starting struct{}
type Stop struct{}
type Stopping struct{}
type Stopped struct {
	Who ActorRef
}
type Watch struct {
	Who ActorRef
}
