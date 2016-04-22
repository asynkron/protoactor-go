package gam

import "sync/atomic"

var node = "nonnode"
var host = "nonhost"
var processDirectory = make(map[uint64]ActorRef)
var sequenceID uint64

func SpawnFunc(producer ActorProducer) *PID {
	props := Props(producer)
	_, pid := spawnChild(props, nil)
	return pid
}

func SpawnTemplate(template Actor) *PID {
	producer := func() Actor {
		return template
	}
	props := Props(producer)
	_, pid := spawnChild(props, nil)
	return pid
}

func Spawn(props Properties) *PID {
	_, pid := spawnChild(props, nil)
	return pid
}

func ActorOf(props Properties) ActorRef {
	ref, _ := spawnChild(props, nil)
	return ref
}

func registerPID(actorRef ActorRef) *PID {
	id := atomic.AddUint64(&sequenceID, 1)

	pid := PID{
		Node: node,
		Host: host,
		Id:   id,
	}

	processDirectory[pid.Id] = actorRef
	return &pid
}

func Tell(pid *PID, message interface{}) {
	ref, _ := FromPID(pid)
	ref.Tell(message)
}

func (pid *PID) SendSystemMessage (message SystemMessage) {
	ref, _ := FromPID(pid)
	ref.SendSystemMessage(message)
}

func (pid *PID) Stop() {
	ref, _ := FromPID(pid)
	ref.Stop()
}

func (pid *PID) Suspend() {
	ref, _ := FromPID(pid)
	ref.(*LocalActorRef).Suspend()
}

func (pid *PID) Resume() {
	ref, _ := FromPID(pid)
	ref.(*LocalActorRef).Resume()
}


func FromPID(pid *PID) (ActorRef, bool) {
	if pid.Host != host || pid.Node != node {
		return deadLetter, false
	}
	ref, ok := processDirectory[pid.Id]
	if !ok {
		return deadLetter, false
	}
	return ref, true
}

func spawnChild(props Properties, parent *PID) (ActorRef, *PID) {
	cell := NewActorCell(props, parent)
	mailbox := props.ProduceMailbox()
	mailbox.RegisterHandlers(cell.invokeUserMessage, cell.invokeSystemMessage)
	ref := NewLocalActorRef(mailbox)	
	pid := registerPID(ref)
	cell.self = pid
	cell.invokeUserMessage(Started{})
	return ref, pid
}
