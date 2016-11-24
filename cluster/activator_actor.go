package cluster

import (
	"log"
	"math/rand"

	"github.com/AsynkronIT/gam/actor"
	"github.com/AsynkronIT/gam/cluster/messages"
)

type activator struct {
}

var activatorPid = actor.SpawnNamed(actor.FromProducer(newActivatorActor()), "activator")

func activatorForHost(host string) *actor.PID {
	pid := actor.NewPID(host, "activator")
	return pid
}

func getRandomActivator() *actor.PID {
	r := rand.Int()
	members := list.Members()
	i := r % len(members)
	member := members[i]
	return activatorForHost(member.Name)
}

func newActivatorActor() actor.Producer {
	return func() actor.Actor {
		return &activator{}
	}
}

func (*activator) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case *actor.Started:
		log.Println("[CLUSTER] Activator actor started")
	case *messages.ActorPidRequest:
		log.Printf("[CLUSTER] Cluster actor creating %v of type %v", msg.Name, msg.Kind)
		props := nameLookup[msg.Kind]
		//pid := actor.SpawnNamed(props.WithReceivers(plugin.Use(&PassivationPlugin{Duration: 5 * time.Second})), msg.Name)
		pid := actor.SpawnNamed(props, msg.Name)
		response := &messages.ActorPidResponse{
			Pid: pid,
		}
		context.Sender().Tell(response)
	default:
		log.Printf("[CLUSTER] Activator got unknown message %+v", msg)
	}
}
