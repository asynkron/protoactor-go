package cluster

// import (
// 	"log"

// 	"github.com/AsynkronIT/protoactor-go/actor"
// 	"github.com/AsynkronIT/protoactor-go/remoting"
// )

// type partitionMasterActor struct {
// 	kindMap map[string]*actor.PID
// }

// func newPartitionMasterActor() actor.Producer {
// 	return func() actor.Actor {
// 		return &partitionMasterActor{
// 			kindMap: make(map[string]*actor.PID),
// 		}
// 	}
// }

// var (
// 	partitionMasterPID = spawnPartitionMasterActor()
// )

// func spawnPartitionMasterActor() *actor.PID {
// 	partitionPid := actor.SpawnNamed(actor.FromProducer(newPartitionMasterActor()), "#partition-master")
// 	return partitionPid
// }

// func (state *partitionMasterActor) Receive(context actor.Context) {
// 	switch msg := context.Message().(type) {
// 	case *actor.Started:
// 		log.Println("[CLUSTER] Partition-Master started")
// 	case *remoting.ActorPidRequest:
// 		//route the request to the correct partition-kind actor
// 		kindPID := state.kindMap[msg.Kind]
// 		kindPID.Tell(msg)
// 	case *MemberJoinedEvent:
// 		//if the new member has any kind's that map to our own kinds, forward the message to the correct partition-kind actor
// 		for _, kind := range msg.Kinds {
// 			kindPID := state.kindMap[kind]
// 			if kindPID != nil {
// 				kindPID.Tell(msg)
// 			}
// 		}
// 	case *MemberLeftEvent:
// 		log.Printf("[CLUSTER] Node left %v", msg.Name())
// 	case *TakeOwnership:
// 	//	state.takeOwnership(msg)
// 	default:
// 		log.Printf("[CLUSTER] Partition-Master got unknown message %+v", msg)
// 	}
// }
