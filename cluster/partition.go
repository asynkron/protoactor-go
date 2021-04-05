package cluster

import (
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/eventstream"
	"github.com/AsynkronIT/protoactor-go/log"
	"github.com/AsynkronIT/protoactor-go/remote"
)

type partitionValue struct {
	kindPIDMap        map[string]*actor.PID
	partitionKindsSub *eventstream.Subscription
	cluster           *Cluster
}

func setupPartition(cluster *Cluster, kinds []string) *partitionValue {
	partition := &partitionValue{
		kindPIDMap: make(map[string]*actor.PID),
		cluster:    cluster,
	}

	for _, kind := range kinds {
		kindPID := partition.spawnPartitionActor(kind)
		partition.kindPIDMap[kind] = kindPID
	}

	partition.partitionKindsSub = cluster.ActorSystem.EventStream.Subscribe(func(m interface{}) {
		if mse, ok := m.(MemberStatusEvent); ok {
			for _, k := range mse.GetKinds() {
				kindPID := partition.kindPIDMap[k]
				if kindPID != nil {
					cluster.ActorSystem.Root.Send(kindPID, m)
				}
			}
		}
	})

	return partition
}

func (p *partitionValue) stopPartition() {
	for _, kindPID := range p.kindPIDMap {
		p.cluster.ActorSystem.Root.StopFuture(kindPID).Wait()
	}
	p.cluster.ActorSystem.EventStream.Unsubscribe(p.partitionKindsSub)
}

func (p *partitionValue) partitionForKind(address, kind string) *actor.PID {
	pid := actor.NewPID(address, "partition-"+kind)
	return pid
}

type spawningProcess struct {
	*actor.Future
	spawningAddress string
}

type partitionActor struct {
	partition      map[string]*actor.PID // actor/grain name to PID
	partitionValue *partitionValue
	keyNameMap     map[string]string           // actor/grain key to name
	spawnings      map[string]*spawningProcess // spawning actor/grain futures
	kind           string
}

func (p *partitionValue) spawnPartitionActor(kind string) *actor.PID {
	props := actor.PropsFromProducer(newPartitionActor(p, kind)).WithGuardian(actor.RestartingSupervisorStrategy())
	partitionPid, _ := p.cluster.ActorSystem.Root.SpawnNamed(props, "partition-"+kind)
	return partitionPid
}

func newPartitionActor(p *partitionValue, kind string) actor.Producer {
	return func() actor.Actor {
		return &partitionActor{
			partition:      make(map[string]*actor.PID),
			keyNameMap:     make(map[string]string),
			spawnings:      make(map[string]*spawningProcess),
			kind:           kind,
			partitionValue: p,
		}
	}
}

func (state *partitionActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case *actor.Started:
		plog.Debug("Started", log.String("kind", state.kind), log.String("id", context.Self().Id))
	case *remote.ActorPidRequest:
		state.spawn(msg, context)
	case *actor.Terminated:
		state.terminated(msg)
	case *TakeOwnership:
		state.takeOwnership(msg, context)
	case *MemberJoinedEvent:
		state.memberJoined(msg, context)
	case *MemberRejoinedEvent:
		state.memberRejoined(msg, context)
	case *MemberLeftEvent:
		state.memberLeft(msg, context)
	case *MemberAvailableEvent:
		plog.Info("Member available", log.String("kind", state.kind), log.String("name", msg.Name()))
	case *MemberUnavailableEvent:
		plog.Info("Member unavailable", log.String("kind", state.kind), log.String("name", msg.Name()))
	case actor.SystemMessage, actor.AutoReceiveMessage:
		// ignore
	default:
		plog.Error("Partition got unknown message", log.String("kind", state.kind), log.TypeOf("type", msg), log.Object("msg", msg))
	}
}

func (state *partitionActor) spawn(msg *remote.ActorPidRequest, context actor.Context) {
	// Check if exist in current partition dictionary
	pid := state.partition[msg.Name]
	if pid != nil {
		response := &remote.ActorPidResponse{Pid: pid}
		context.Respond(response)
		return
	}

	// Check if is spawning, if so just await spawning finish.
	spawning := state.spawnings[msg.Name]
	if spawning != nil {
		context.AwaitFuture(spawning.Future, func(r interface{}, err error) {
			response, ok := r.(*remote.ActorPidResponse)
			if !ok {
				context.Respond(remote.ActorPidRespErr)
				return
			}
			context.Respond(response)
		})
		return
	}

	// Get activator
	activator := state.partitionValue.cluster.MemberList.getActivatorMember(msg.Kind)
	if activator == "" {
		// No activator currently available, return unavailable
		context.Respond(remote.ActorPidRespUnavailable)
		return
	}

	// Create SpawningProcess and cache it in spawnings dictionary.
	spawning = &spawningProcess{actor.NewFuture(context.ActorSystem(), -1), activator}
	state.spawnings[msg.Name] = spawning

	// Await SpawningProcess
	context.AwaitFuture(spawning.Future, func(r interface{}, err error) {
		delete(state.spawnings, msg.Name)

		// Check if exist in current partition dictionary
		// This is necessary to avoid race condition during partition map transferring.
		pid = state.partition[msg.Name]
		if pid != nil {
			response := &remote.ActorPidResponse{Pid: pid}
			context.Respond(response)
			return
		}

		response, ok := r.(*remote.ActorPidResponse)
		if !ok {
			context.Respond(remote.ActorPidRespErr)
			return
		}

		if response.StatusCode == remote.ResponseStatusCodeOK.ToInt32() {
			pid = response.Pid
			state.partition[msg.Name] = pid
			state.keyNameMap[pid.String()] = msg.Name
			context.Watch(pid)
		}

		context.Respond(response)
	})

	// Perform Spawning
	go state.spawning(msg, activator, 3, spawning.PID(), context)
}

func (state *partitionActor) spawning(msg *remote.ActorPidRequest, activator string, retryLeft int, fPid *actor.PID, context actor.Context) {
	if activator == "" {
		activator = state.partitionValue.cluster.MemberList.getActivatorMember(msg.Kind)
		if activator == "" {
			// No activator currently available, return unavailable
			context.Send(fPid, remote.ActorPidRespUnavailable)
			return
		}
	}

	pidResp, err := state.partitionValue.cluster.remote.SpawnNamed(activator, msg.Name, msg.Kind, state.partitionValue.cluster.Config.TimeoutTime)
	if err != nil {
		plog.Error("Partition failed to spawn actor", log.String("name", msg.Name), log.String("kind", msg.Kind), log.String("address", activator), log.Error(err))
		if err == actor.ErrTimeout {
			context.Send(fPid, remote.ActorPidRespTimeout)
		} else {
			context.Send(fPid, remote.ActorPidRespErr)
		}
		return
	}

	if pidResp.StatusCode == remote.ResponseStatusCodeUNAVAILABLE.ToInt32() && retryLeft != 0 {
		retryLeft--
		state.spawning(msg, "", retryLeft, fPid, context)
		return
	}

	context.Send(fPid, pidResp)
}

func (state *partitionActor) terminated(msg *actor.Terminated) {
	// one of the actors we manage died, remove it from the lookup
	key := msg.Who.String()
	if name, ok := state.keyNameMap[key]; ok {
		delete(state.partition, name)
		delete(state.keyNameMap, key)
	}
}

func (state *partitionActor) memberRejoined(msg *MemberRejoinedEvent, context actor.Context) {
	memberAddress := msg.Name()

	plog.Info("Member rejoined", log.String("kind", state.kind), log.String("name", memberAddress))

	for actorID, pid := range state.partition {
		// if the mapped PID is on the address that left, forget it
		if pid.Address == memberAddress {
			//	log.Printf("[CLUSTER] Forgetting '%v' - '%v'", actorID, memberAddress)
			delete(state.partition, actorID)
			delete(state.keyNameMap, pid.String())
		}
	}

	// Process Spawning Process
	for _, spawning := range state.spawnings {
		if spawning.spawningAddress == memberAddress {
			context.Send(spawning.PID(), remote.ActorPidRespUnavailable)
		}
	}
}

func (state *partitionActor) memberLeft(msg *MemberLeftEvent, context actor.Context) {
	memberAddress := msg.Name()

	plog.Info("Member left", log.String("kind", state.kind), log.String("name", memberAddress))

	// If the left member is self, transfer remaining pids to others
	if state.partitionValue.cluster.ActorSystem.Address() == memberAddress {
		for actorID := range state.partition {
			address := state.partitionValue.cluster.MemberList.getPartitionMember(actorID, state.kind)
			if address != "" {
				state.transferOwnership(actorID, address, context)
			}
		}
	}

	for actorID, pid := range state.partition {
		// if the mapped PID is on the address that left, forget it
		if pid.Address == memberAddress {
			//	log.Printf("[CLUSTER] Forgetting '%v' - '%v'", actorID, memberAddress)
			delete(state.partition, actorID)
			delete(state.keyNameMap, pid.String())
		}
	}

	// Process Spawning Process
	for _, spawning := range state.spawnings {
		if spawning.spawningAddress == memberAddress {
			context.Send(spawning.PID(), remote.ActorPidRespUnavailable)
		}
	}
}

func (state *partitionActor) memberJoined(msg *MemberJoinedEvent, context actor.Context) {
	plog.Debug("Member joined", log.String("kind", state.kind), log.String("name", msg.Name()))
	for actorID := range state.partition {
		address := state.partitionValue.cluster.MemberList.getPartitionMember(actorID, state.kind)
		if address != "" && address != state.partitionValue.cluster.ActorSystem.Address() {
			state.transferOwnership(actorID, address, context)
		}
	}
	for actorID, spawning := range state.spawnings {
		address := state.partitionValue.cluster.MemberList.getPartitionMember(actorID, state.kind)
		if address != "" && address != state.partitionValue.cluster.ActorSystem.Address() {
			context.Send(spawning.PID(), remote.ActorPidRespUnavailable)
		}
	}
}

func (state *partitionActor) transferOwnership(actorID string, address string, context actor.Context) {
	pid := state.partition[actorID]
	owner := state.partitionValue.partitionForKind(address, state.kind)
	context.Send(owner, &TakeOwnership{
		Pid:  pid,
		Name: actorID,
	})
	// we can safely delete this entry as the consistent hash no longer points to us
	delete(state.partition, actorID)
	delete(state.keyNameMap, pid.String())
	context.Unwatch(pid)
}

func (state *partitionActor) takeOwnership(msg *TakeOwnership, context actor.Context) {
	// Check again if I'm the owner
	address := state.partitionValue.cluster.MemberList.getPartitionMember(msg.Name, state.kind)
	if address != "" && address != state.partitionValue.cluster.ActorSystem.Address() {
		// if not, forward to the correct owner
		owner := state.partitionValue.partitionForKind(address, state.kind)
		context.Send(owner, msg)
		return
	}
	// Cache ownership
	state.partition[msg.Name] = msg.Pid
	state.keyNameMap[msg.Pid.String()] = msg.Name
	context.Watch(msg.Pid)
}
