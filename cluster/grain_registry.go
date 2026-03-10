package cluster

import (
	"errors"
	"strings"

	"github.com/asynkron/protoactor-go/actor"
)

// ErrEnumerationNotSupported is returned by GrainRegistry methods that require
// the IdentityLookup to implement GrainEnumerator.
var ErrEnumerationNotSupported = errors.New("identity lookup does not support grain enumeration")

// GrainRegistry provides read-only access to grain activation information.
type GrainRegistry struct {
	cluster *Cluster
}

// Count returns the total number of active grains on the local node.
func (r *GrainRegistry) Count() int {
	return int(r.cluster.VirtualActorCount())
}

// CountByKind returns a map of kind name to active grain count on the local node.
func (r *GrainRegistry) CountByKind() map[string]int {
	r.cluster.kindsMu.RLock()
	defer r.cluster.kindsMu.RUnlock()

	result := make(map[string]int, len(r.cluster.kinds))
	for name, ak := range r.cluster.kinds {
		result[name] = int(ak.Count())
	}
	return result
}

func (r *GrainRegistry) enumerator() (GrainEnumerator, error) {
	if enum, ok := r.cluster.IdentityLookup.(GrainEnumerator); ok {
		return enum, nil
	}
	return nil, ErrEnumerationNotSupported
}

// All returns all known grain activations. Returns ErrEnumerationNotSupported
// if the IdentityLookup does not implement GrainEnumerator.
func (r *GrainRegistry) All() ([]*GrainInfo, error) {
	enum, err := r.enumerator()
	if err != nil {
		return nil, err
	}
	grains, err := enum.ListGrains()
	if err != nil {
		return nil, err
	}
	r.enrichWithMetrics(grains)
	return grains, nil
}

// ByKind returns grain activations filtered by kind. Returns ErrEnumerationNotSupported
// if the IdentityLookup does not implement GrainEnumerator.
func (r *GrainRegistry) ByKind(kind string) ([]*GrainInfo, error) {
	enum, err := r.enumerator()
	if err != nil {
		return nil, err
	}
	grains, err := enum.ListGrainsByKind(kind)
	if err != nil {
		return nil, err
	}
	r.enrichWithMetrics(grains)
	return grains, nil
}

// ByMember returns grain activations owned by a specific member. Returns
// ErrEnumerationNotSupported if the IdentityLookup does not implement GrainEnumerator.
func (r *GrainRegistry) ByMember(memberID string) ([]*GrainInfo, error) {
	enum, err := r.enumerator()
	if err != nil {
		return nil, err
	}
	grains, err := enum.ListGrainsByMember(memberID)
	if err != nil {
		return nil, err
	}
	r.enrichWithMetrics(grains)
	return grains, nil
}

// Get returns a single grain activation by identity and kind. Returns
// ErrEnumerationNotSupported if the IdentityLookup does not implement GrainEnumerator.
// Note: This is O(N) in the number of grains of the given kind, as it fetches
// all grains and filters. Intended for admin/diagnostic use, not hot paths.
func (r *GrainRegistry) Get(identity, kind string) (*GrainInfo, bool, error) {
	enum, err := r.enumerator()
	if err != nil {
		return nil, false, err
	}
	grains, err := enum.ListGrainsByKind(kind)
	if err != nil {
		return nil, false, err
	}
	for _, g := range grains {
		if g.Identity == identity {
			r.enrichWithMetrics([]*GrainInfo{g})
			return g, true, nil
		}
	}
	return nil, false, nil
}

func (r *GrainRegistry) enrichWithMetrics(grains []*GrainInfo) {
	if r.cluster.grainMetrics == nil {
		return
	}
	for _, g := range grains {
		key := g.Kind + "/" + g.Identity
		r.cluster.grainMetrics.applyTo(key, g)
	}
}

// ParseStoredActivationInfoKey parses a key in "kind/identity" format.
func ParseStoredActivationInfoKey(key string) (kind, identity string) {
	if idx := strings.Index(key, "/"); idx >= 0 {
		return key[:idx], key[idx+1:]
	}
	return key, ""
}

// ParseDotSeparatedKey parses a key in "kind.identity" format (used by NATS).
func ParseDotSeparatedKey(key string) (kind, identity string) {
	if idx := strings.Index(key, "."); idx >= 0 {
		return key[:idx], key[idx+1:]
	}
	return key, ""
}

// StoredActivationInfoToGrainInfo converts a StoredActivationInfo to a GrainInfo.
func StoredActivationInfoToGrainInfo(info *StoredActivationInfo) *GrainInfo {
	var pid *actor.PID
	if info.Pid != "" {
		if idx := strings.Index(info.Pid, "/"); idx >= 0 {
			pid = actor.NewPID(info.Pid[:idx], info.Pid[idx+1:])
		}
	}
	return &GrainInfo{
		Identity: info.Identity,
		Kind:     info.Kind,
		PID:      pid,
		MemberID: info.MemberID,
	}
}
