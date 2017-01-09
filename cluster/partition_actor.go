package cluster

import (
	"log"
	"time"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/remoting"
)

var (
	kindPIDMap map[string]*actor.PID
)

func subscribePartitionKindsToEventStream() {
	actor.EventStream.Subscribe(func(m interface{}) {
		if mse, ok := m.(MemberStatusEvent); ok {
			for _, k := range mse.GetKinds() {
				kindPID := kindPIDMap[k]
				if kindPID != nil {
					kindPID.Tell(m)
				}
			}
		}
	})
}

func spawnPartitionActor(kind string) *actor.PID {
	partitionPid := actor.SpawnNamed(actor.FromProducer(newPartitionActor(kind)), "#partition-"+kind)
	return partitionPid
}

func partitionForKind(host, kind string) *actor.PID {
	pid := actor.NewPID(host, "#partition-"+kind)
	return pid
}

func newPartitionActor(kind string) actor.Producer {
	return func() actor.Actor {
		return &partitionActor{
			partition: make(map[string]*actor.PID),
			kind:      kind,
		}
	}
}

type partitionActor struct {
	partition map[string]*actor.PID //actor/grain name to PID
	kind      string
}

func (state *partitionActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case *actor.Started:
		log.Printf("[CLUSTER] Started %v", context.Self().Id)
	case *remoting.ActorPidRequest:
		state.spawn(msg, context)
	case *MemberJoinedEvent:
		state.memberJoined(msg)
	case *MemberRejoinedEvent:
		state.memberRejoined(msg)
	case *MemberLeftEvent:
		state.memberLeft(msg)
	case *MemberAvailableEvent:
		log.Printf("[CLUSTER] Node Available %v", msg.Name())
	case *MemberUnavailableEvent:
		log.Printf("[CLUSTER] Node Unavailable %v", msg.Name())
	case *TakeOwnership:

		state.takeOwnership(msg)
	default:
		log.Printf("[CLUSTER] Partition got unknown message %+v", msg)
	}
}

func (state *partitionActor) spawn(msg *remoting.ActorPidRequest, context actor.Context) {

	//TODO: make this async
	pid := state.partition[msg.Name]
	if pid == nil {
		//get a random node
		random := getRandomActivator(msg.Kind)
		var err error
		pid, err = remoting.Spawn(random, msg.Name, msg.Kind, 5*time.Second)
		if err != nil {
			log.Printf("[CLUSTER] Partition failed to spawn '%v' of kind '%v' on host '%v'", msg.Name, msg.Kind, random)
			return
		}
		state.partition[msg.Name] = pid
	}
	response := &remoting.ActorPidResponse{
		Pid: pid,
	}
	context.Respond(response)
}

func (state *partitionActor) memberRejoined(msg *MemberRejoinedEvent) {
	log.Printf("[CLUSTER] Node Rejoined %v", msg.Name())
	for actorID, pid := range state.partition {
		//if the mapped PID is on the host that left, forget it
		if pid.Host == msg.Name() {
			//	log.Printf("[CLUSTER] Forgetting '%v' - '%v'", actorID, msg.Name())
			delete(state.partition, actorID)
		}
	}
}

func (state *partitionActor) memberLeft(msg *MemberLeftEvent) {
	log.Printf("[CLUSTER] Node Left %v", msg.Name())
	for actorID, pid := range state.partition {
		//if the mapped PID is on the host that left, forget it
		if pid.Host == msg.Name() {
			//	log.Printf("[CLUSTER] Forgetting '%v' - '%v'", actorID, msg.Name())
			delete(state.partition, actorID)
		}
	}
}

func (state *partitionActor) memberJoined(msg *MemberJoinedEvent) {
	log.Printf("[CLUSTER] Node Joined %v", msg.Name())
	for actorID := range state.partition {
		host := getNode(actorID, state.kind)
		if host != actor.ProcessRegistry.Host {
			state.transferOwnership(actorID, host)
		}
	}
}

func (state *partitionActor) transferOwnership(actorID string, host string) {
	log.Printf("[CLUSTER] Giving ownership of %v to Node %v", actorID, host)
	pid := state.partition[actorID]
	owner := partitionForKind(host, state.kind)
	owner.Tell(&TakeOwnership{
		Pid:  pid,
		Name: actorID,
	})
	//we can safely delete this entry as the consisntent hash no longer points to us
	delete(state.partition, actorID)
}

func (state *partitionActor) takeOwnership(msg *TakeOwnership) {
	log.Printf("[CLUSTER] Took ownerhip of %v", msg.Pid)
	state.partition[msg.Name] = msg.Pid
}
