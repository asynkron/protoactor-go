package cluster

import "fmt"

// PeekStatus indicates the liveness state of a grain.
type PeekStatus int

const (
	// PeekStatusNotFound means no activation record exists.
	PeekStatusNotFound PeekStatus = iota
	// PeekStatusAlive means the activation record exists, the owning member
	// is in the cluster, and the placement actor confirmed the process is running.
	PeekStatusAlive
	// PeekStatusMemberDead means an activation record exists but the owning
	// member is no longer in the cluster's member list.
	PeekStatusMemberDead
	// PeekStatusStale means an activation record exists and the owning member
	// is alive, but the placement actor reports no such process. The record
	// is orphaned.
	PeekStatusStale
)

func (s PeekStatus) String() string {
	switch s {
	case PeekStatusNotFound:
		return "not_found"
	case PeekStatusAlive:
		return "alive"
	case PeekStatusMemberDead:
		return "member_dead"
	case PeekStatusStale:
		return "stale"
	default:
		return fmt.Sprintf("PeekStatus(%d)", int(s))
	}
}

// PeekResult contains the result of a non-activating grain liveness check.
type PeekResult struct {
	*GrainInfo
	Status PeekStatus
}
