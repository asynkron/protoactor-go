package actor

//system messages
type Starting struct{}
type Stop struct{}
type Watch struct {
	Watcher ActorRef
}
type Unwatch struct {
	Watcher ActorRef
}
type OtherStopped struct {
	Who ActorRef
}
type CreateActor struct {
	Props   PropsValue
	ReplyTo ActorRef
}
type Failure struct {
	Who    ActorRef
	Reason interface{}
}
type Restart struct {}

//user message
type Stopping struct{}

type Stopped struct{}
type PoisonPill struct{}
