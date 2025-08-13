package test

import (
	"log/slog"
	"sync"
	"time"

	"github.com/asynkron/protoactor-go/cluster"
	"golang.org/x/exp/maps"
)

// ProviderConfig holds configuration values for the in-memory test provider.
type ProviderConfig struct {
	// ServiceTTL is the time to live for services. Default: 3s
	ServiceTTL time.Duration
	// RefreshTTL is the time between refreshes of the service ttl. Default: 1s
	RefreshTTL time.Duration
	// DeregisterCritical is the time after which a service is deregistered if it is not refreshed. Default: 10s
	DeregisterCritical time.Duration
}

// ProviderOption configures a test provider instance.
type ProviderOption func(config *ProviderConfig)

// WithTestProviderServiceTTL sets the service ttl. Default: 3s
func WithTestProviderServiceTTL(serviceTTL time.Duration) ProviderOption {
	return func(config *ProviderConfig) {
		config.ServiceTTL = serviceTTL
	}
}

// WithTestProviderRefreshTTL sets the refresh ttl. Default: 1s
func WithTestProviderRefreshTTL(refreshTTL time.Duration) ProviderOption {
	return func(config *ProviderConfig) {
		config.RefreshTTL = refreshTTL
	}
}

// WithTestProviderDeregisterCritical sets the deregister critical. Default: 10s
func WithTestProviderDeregisterCritical(deregisterCritical time.Duration) ProviderOption {
	return func(config *ProviderConfig) {
		config.DeregisterCritical = deregisterCritical
	}
}

// Provider implements cluster.ClusterProvider using an in-memory agent.
type Provider struct {
	memberList *cluster.MemberList
	config     *ProviderConfig

	agent           *InMemAgent
	id              string
	ttlReportTicker *time.Ticker
	cluster         *cluster.Cluster
}

// NewTestProvider creates a new Provider backed by the given in-memory agent.
func NewTestProvider(agent *InMemAgent, options ...ProviderOption) *Provider {
	config := &ProviderConfig{
		ServiceTTL:         time.Second * 3,
		RefreshTTL:         time.Second,
		DeregisterCritical: time.Second * 10,
	}
	for _, option := range options {
		option(config)
	}
	return &Provider{
		config: config,
		agent:  agent,
	}
}

// StartMember starts the provider as a cluster member.
func (t *Provider) StartMember(c *cluster.Cluster) error {

	c.ActorSystem.Logger().Debug("start cluster member")
	t.memberList = c.MemberList
	host, port, err := c.ActorSystem.GetHostPort()
	if err != nil {
		return err
	}
	kinds := c.GetClusterKinds()
	t.cluster = c
	t.id = c.ActorSystem.ID
	t.startTTLReport()
	t.agent.SubscribeStatusUpdate(t.notifyStatuses)
	t.agent.RegisterService(NewAgentServiceStatus(t.id, host, port, kinds))
	return nil
}

// StartClient starts the provider in client mode without registering services.
func (t *Provider) StartClient(cluster *cluster.Cluster) error {
	t.memberList = cluster.MemberList
	t.id = cluster.ActorSystem.ID
	t.agent.SubscribeStatusUpdate(t.notifyStatuses)
	t.agent.ForceUpdate()
	return nil
}

// Shutdown stops the provider and deregisters its service.
func (t *Provider) Shutdown(_ bool) error {
	t.cluster.Logger().Debug("Unregistering service", slog.String("service", t.id))
	if t.ttlReportTicker != nil {
		t.ttlReportTicker.Stop()
	}
	t.agent.DeregisterService(t.id)
	return nil
}

// notifyStatuses notifies the cluster that the service status has changed.
func (t *Provider) notifyStatuses() {
	statuses := t.agent.GetStatusHealth()

	t.cluster.Logger().Debug("TestAgent response", slog.Any("statuses", statuses))
	members := make([]*cluster.Member, 0, len(statuses))
	for _, status := range statuses {
		copiedKinds := make([]string, 0, len(status.Kinds))
		copiedKinds = append(copiedKinds, status.Kinds...)

		members = append(members, &cluster.Member{
			Id:    status.ID,
			Port:  int32(status.Port),
			Host:  status.Host,
			Kinds: copiedKinds,
		})
	}
	t.memberList.UpdateClusterTopology(members)
}

// startTTLReport starts the ttl report loop.
func (t *Provider) startTTLReport() {
	t.ttlReportTicker = time.NewTicker(t.config.RefreshTTL)
	go func() {
		for range t.ttlReportTicker.C {
			t.agent.RefreshServiceTTL(t.id)
		}
	}()
}

// InMemAgent is a lightweight service registry used for testing.
type InMemAgent struct {
	services     map[string]AgentServiceStatus
	servicesLock *sync.RWMutex

	statusUpdateHandlers     []func()
	statusUpdateHandlersLock *sync.RWMutex
}

// NewInMemAgent returns a new in-memory agent.
func NewInMemAgent() *InMemAgent {
	return &InMemAgent{
		services:                 make(map[string]AgentServiceStatus),
		servicesLock:             &sync.RWMutex{},
		statusUpdateHandlers:     make([]func(), 0),
		statusUpdateHandlersLock: &sync.RWMutex{},
	}
}

// RegisterService registers a AgentServiceStatus with the agent.
func (m *InMemAgent) RegisterService(registration AgentServiceStatus) {
	m.servicesLock.Lock()
	m.services[registration.ID] = registration
	m.servicesLock.Unlock()

	m.onStatusUpdate()
}

// DeregisterService removes a service from the agent.
func (m *InMemAgent) DeregisterService(id string) {
	m.servicesLock.Lock()
	delete(m.services, id)
	m.servicesLock.Unlock()

	m.onStatusUpdate()
}

// RefreshServiceTTL updates the TTL of all services.
func (m *InMemAgent) RefreshServiceTTL(id string) {
	m.servicesLock.Lock()
	defer m.servicesLock.Unlock()
	if service, ok := m.services[id]; ok {
		service.TTL = time.Now()
		m.services[id] = service
	}
}

// SubscribeStatusUpdate registers a handler that will be called when the service map changes.
func (m *InMemAgent) SubscribeStatusUpdate(handler func()) {
	m.statusUpdateHandlersLock.Lock()
	defer m.statusUpdateHandlersLock.Unlock()
	m.statusUpdateHandlers = append(m.statusUpdateHandlers, handler)
}

// GetStatusHealth returns the health of the service.
func (m *InMemAgent) GetStatusHealth() []AgentServiceStatus {
	m.servicesLock.RLock()
	defer m.servicesLock.RUnlock()
	return maps.Values(m.services)
}

// ForceUpdate is used to trigger a status update event.
func (m *InMemAgent) ForceUpdate() {
	m.onStatusUpdate()
}

func (m *InMemAgent) onStatusUpdate() {
	m.statusUpdateHandlersLock.RLock()
	defer m.statusUpdateHandlersLock.RUnlock()
	for _, handler := range m.statusUpdateHandlers {
		handler()
	}
}

// AgentServiceStatus contains metadata about a registered service.
type AgentServiceStatus struct {
	ID    string
	TTL   time.Time // last alive time
	Host  string
	Port  int
	Kinds []string
}

// NewAgentServiceStatus creates a new AgentServiceStatus.
func NewAgentServiceStatus(id string, host string, port int, kinds []string) AgentServiceStatus {
	return AgentServiceStatus{
		ID:    id,
		TTL:   time.Now(),
		Host:  host,
		Port:  port,
		Kinds: kinds,
	}
}

// Alive reports whether the service TTL has not expired.
func (a AgentServiceStatus) Alive() bool {
	return time.Since(a.TTL) <= 5*time.Second
}
