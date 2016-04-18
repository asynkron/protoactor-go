package actor

//system messages
type Starting struct{}
type Stop struct{}
type Watch struct {
	Who ActorRef
}
type WatchedStopped struct {
	Who ActorRef
}
type CreateActor struct {
	Props   PropsValue
	ReplyTo ActorRef
}

//user message
type Stopping struct{}

type Stopped struct{}
type PoisonPill struct{}
type Kill struct{}
