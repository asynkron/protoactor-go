package cluster

import (
	"sync"

	"github.com/awevoke/protoactor-go/actor"
)

// GrainActivated is published to the EventStream when a grain actor starts.
type GrainActivated struct {
	ClusterIdentity *ClusterIdentity
	PID             *actor.PID
}

// DeactivationReason indicates why a grain was deactivated.
type DeactivationReason int

const (
	DeactivationReasonUnknown DeactivationReason = iota
	DeactivationReasonPassivation
	DeactivationReasonShutdown
	DeactivationReasonTopologyChange
)

func (r DeactivationReason) String() string {
	switch r {
	case DeactivationReasonPassivation:
		return "passivation"
	case DeactivationReasonShutdown:
		return "shutdown"
	case DeactivationReasonTopologyChange:
		return "topology-change"
	default:
		return "unknown"
	}
}

// GrainDeactivated is published to the EventStream when a grain actor stops.
type GrainDeactivated struct {
	ClusterIdentity *ClusterIdentity
	PID             *actor.PID
	Reason          DeactivationReason
}

// deactivationReasons tracks the reason a grain is being deactivated.
type deactivationReasons struct {
	m sync.Map // map[pidKey]DeactivationReason
}

func newDeactivationReasons() *deactivationReasons {
	return &deactivationReasons{}
}

func pidKey(pid *actor.PID) string {
	return pid.Address + "/" + pid.Id
}

func (d *deactivationReasons) Set(pid *actor.PID, reason DeactivationReason) {
	d.m.Store(pidKey(pid), reason)
}

func (d *deactivationReasons) Pop(pid *actor.PID) DeactivationReason {
	key := pidKey(pid)
	v, ok := d.m.LoadAndDelete(key)
	if !ok {
		return DeactivationReasonUnknown
	}
	return v.(DeactivationReason)
}
