package test

import (
	"sync"
	"time"

	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/log"
	"golang.org/x/exp/maps"
)

type ProviderConfig struct {
	// ServiceTtl is the time to live for services. Default: 3s
	ServiceTtl time.Duration
	// RefreshTtl is the time between refreshes of the service ttl. Default: 1s
	RefreshTtl time.Duration
	// DeregisterCritical is the time after which a service is deregistered if it is not refreshed. Default: 10s
	DeregisterCritical time.Duration
}

type ProviderOption func(config *ProviderConfig)

// WithTestProviderServiceTtl sets the service ttl. Default: 3s
func WithTestProviderServiceTtl(serviceTtl time.Duration) ProviderOption {
	return func(config *ProviderConfig) {
		config.ServiceTtl = serviceTtl
	}
}

// WithTestProviderRefreshTtl sets the refresh ttl. Default: 1s
func WithTestProviderRefreshTtl(refreshTtl time.Duration) ProviderOption {
	return func(config *ProviderConfig) {
		config.RefreshTtl = refreshTtl
	}
}

// WithTestProviderDeregisterCritical sets the deregister critical. Default: 10s
func WithTestProviderDeregisterCritical(deregisterCritical time.Duration) ProviderOption {
	return func(config *ProviderConfig) {
		config.DeregisterCritical = deregisterCritical
	}
}

type Provider struct {
	memberList *cluster.MemberList
	config     *ProviderConfig

	agent           *InMemAgent
	id              string
	ttlReportTicker *time.Ticker
}

func NewTestProvider(agent *InMemAgent, options ...ProviderOption) *Provider {
	config := &ProviderConfig{
		ServiceTtl:         time.Second * 3,
		RefreshTtl:         time.Second,
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

func (t *Provider) StartMember(c *cluster.Cluster) error {
	plog.Debug("start cluster member")
	t.memberList = c.MemberList
	host, port, err := c.ActorSystem.GetHostPort()
	if err != nil {
		return err
	}
	kinds := c.GetClusterKinds()
	t.id = c.ActorSystem.ID
	t.startTtlReport()
	t.agent.SubscribeStatusUpdate(t.notifyStatuses)
	t.agent.RegisterService(NewAgentServiceStatus(t.id, host, port, kinds))
	return nil
}

func (t *Provider) StartClient(cluster *cluster.Cluster) error {
	t.memberList = cluster.MemberList
	t.id = cluster.ActorSystem.ID
	t.agent.SubscribeStatusUpdate(t.notifyStatuses)
	t.agent.ForceUpdate()
	return nil
}

func (t *Provider) Shutdown(_ bool) error {
	plog.Debug("Unregistering service", log.String("service", t.id))
	if t.ttlReportTicker != nil {
		t.ttlReportTicker.Stop()
	}
	t.agent.DeregisterService(t.id)
	return nil
}

// notifyStatuses notifies the cluster that the service status has changed.
func (t *Provider) notifyStatuses() {
	statuses := t.agent.GetStatusHealth()

	plog.Debug("TestAgent response", log.Object("statuses", statuses))
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

// startTtlReport starts the ttl report loop.
func (t *Provider) startTtlReport() {
	t.ttlReportTicker = time.NewTicker(t.config.RefreshTtl)
	go func() {
		for range t.ttlReportTicker.C {
			t.agent.RefreshServiceTTL(t.id)
		}
	}()
}

type InMemAgent struct {
	services     map[string]AgentServiceStatus
	servicesLock *sync.RWMutex

	statusUpdateHandlers     []func()
	statusUpdateHandlersLock *sync.RWMutex
}

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

func (a AgentServiceStatus) Alive() bool {
	return time.Now().Sub(a.TTL) <= (time.Second * 5)
}
