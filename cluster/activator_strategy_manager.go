package cluster

import "sync"

// StrategyManager dispatches activation placement decisions to per-kind
// ActivatorStrategy instances. Each kind can have its own strategy (set via
// WithActivatorStrategy on the Kind). Kinds without a specific strategy
// use the cluster-default (set via WithDefaultActivatorStrategy).
type StrategyManager struct {
	mu              sync.RWMutex
	cluster         *Cluster
	kindStrategies  map[string]ActivatorStrategy // kind name -> strategy
	defaultStrategy ActivatorStrategy
}

// NewStrategyManager creates a new StrategyManager for the given cluster.
// If the cluster has no DefaultActivatorStrategy configured, a
// RoundRobinStrategy is used as the fallback.
func NewStrategyManager(cluster *Cluster) *StrategyManager {
	var defaultStrategy ActivatorStrategy
	if cluster.Config.DefaultActivatorStrategy != nil {
		defaultStrategy = cluster.Config.DefaultActivatorStrategy(cluster)
	} else {
		defaultStrategy = NewRoundRobinStrategy()
	}

	return &StrategyManager{
		cluster:         cluster,
		kindStrategies:  make(map[string]ActivatorStrategy),
		defaultStrategy: defaultStrategy,
	}
}

// GetActivator selects which member should host the given identity.
// Uses the kind-specific strategy if set, otherwise the default.
func (sm *StrategyManager) GetActivator(ci *ClusterIdentity, senderAddress string) *Member {
	sm.mu.RLock()
	strategy, ok := sm.kindStrategies[ci.Kind]
	sm.mu.RUnlock()

	if !ok || strategy == nil {
		return sm.defaultStrategy.GetActivator(ci, senderAddress)
	}
	return strategy.GetActivator(ci, senderAddress)
}

// AddMember notifies all registered strategies that a member has joined.
func (sm *StrategyManager) AddMember(member *Member) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	sm.defaultStrategy.AddMember(member)
	for _, s := range sm.kindStrategies {
		s.AddMember(member)
	}
}

// RemoveMember notifies all registered strategies that a member has left.
func (sm *StrategyManager) RemoveMember(member *Member) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	sm.defaultStrategy.RemoveMember(member)
	for _, s := range sm.kindStrategies {
		s.RemoveMember(member)
	}
}

// RegisterKindStrategy registers a strategy for a specific kind.
// Called during Setup when kinds are initialized.
func (sm *StrategyManager) RegisterKindStrategy(kind string, strategy ActivatorStrategy) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.kindStrategies[kind] = strategy
}

// Close releases resources for all strategies.
func (sm *StrategyManager) Close() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.defaultStrategy.Close()
	for _, s := range sm.kindStrategies {
		s.Close()
	}
}
