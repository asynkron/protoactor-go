package shared

import (
	"log"

	"github.com/AsynkronIT/gam/actor"
	"github.com/AsynkronIT/gam/cluster"
)

type type1 struct {
}

func (*type1) Receive(context actor.Context) {
	switch context.Message().(type) {
	case *actor.Started:
		log.Println("type1 started")
	case *HelloMessage:
		log.Println("type1 got hello message!!!......")
	}
}

type type2 struct {
}

func (*type2) Receive(context actor.Context) {
	switch context.Message().(type) {
	case *actor.Started:
		log.Println("type1 started")
	}
}

const (
	Type1 = "type1"
	Type2 = "type2"
)

func newType1() actor.Actor {
	return &type1{}
}

func newType2() actor.Actor {
	return &type2{}
}

func init() {
	log.Println("Registering actors...")
	cluster.Register(Type1, actor.FromProducer(newType1))
	cluster.Register(Type2, actor.FromProducer(newType2))
}
