package actor

import (
	"github.com/rogeralsing/goactor/interfaces"
)

type Stop struct{}

func (*Stop) SystemMessage() {}

type Watch struct {
	Watcher interfaces.ActorRef
}

func (*Watch) SystemMessage() {}

type Unwatch struct {
	Watcher interfaces.ActorRef
}

func (*Unwatch) SystemMessage() {}

type OtherStopped struct {
	Who interfaces.ActorRef
}

func (*OtherStopped) SystemMessage() {}

type Failure struct {
	Who    interfaces.ActorRef
	Reason interface{}
}

func (*Failure) SystemMessage() {}

type Restart struct{}

func (*Restart) SystemMessage() {}

