package automanaged

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/net/context"

	"github.com/labstack/echo"
	"github.com/otherview/protoactor-go/cluster"
	"github.com/otherview/protoactor-go/eventstream"
	"github.com/otherview/protoactor-go/log"
	"golang.org/x/sync/errgroup"
)

var (
	plog                       = log.New(log.DebugLevel, "[CLUSTER] [AUTOMANAGED]")
	clusterTTLErrorMutex       = new(sync.Mutex)
	clusterMonitorErrorMutex   = new(sync.Mutex)
	shutdownMutex              = new(sync.Mutex)
	deregisteredMutex          = new(sync.Mutex)
	activeProviderMutex        = new(sync.Mutex)
	activeProviderRunningMutex = new(sync.Mutex)
)

type AutoManagedProvider struct {
	deregistered          bool
	shutdown              bool
	activeProvider        *echo.Echo
	activeProviderRunning bool
	activeProviderTesting bool
	httpClient            *http.Client
	monitoringStatus      bool
	id                    string
	clusterName           string
	address               string
	autoManagePort        int
	memberPort            int
	knownKinds            []string
	knownNodes            []*NodeModel
	hosts                 []string
	refreshTTL            time.Duration
	statusValue           cluster.MemberStatusValue
	statusValueSerializer cluster.MemberStatusValueSerializer
	clusterTTLError       error
	clusterMonitorError   error
}

// New creates a AutoManagedProvider that connects locally
func New() *AutoManagedProvider {
	return NewWithConfig(
		2*time.Second,
		6330,
		"localhost:6330",
	)
}

// NewWithConfig creates an Automanaged Provider that connects to a all the hosts
func NewWithConfig(refreshTTL time.Duration, autoManPort int, hosts ...string) *AutoManagedProvider {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 5 * time.Second,
		}).DialContext,
		MaxIdleConns:          10,
		IdleConnTimeout:       90 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		MaxConnsPerHost:       10,
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   2 * time.Second,
	}

	p := &AutoManagedProvider{
		hosts:                 hosts,
		httpClient:            httpClient,
		refreshTTL:            refreshTTL,
		autoManagePort:        autoManPort,
		activeProviderRunning: false,
		monitoringStatus:      false,
	}

	return p
}

// NewWithTesting creates a testable provider
func NewWithTesting(refreshTTL time.Duration, autoManPort int, activeProvider *echo.Echo, hosts ...string) *AutoManagedProvider {
	p := NewWithConfig(refreshTTL, autoManPort, hosts...)
	p.activeProviderTesting = true
	p.activeProvider = activeProvider

	return p
}

func (p *AutoManagedProvider) RegisterMember(clusterName string, address string, port int, knownKinds []string,
	statusValue cluster.MemberStatusValue, serializer cluster.MemberStatusValueSerializer) error {
	p.id = fmt.Sprintf("%v@%v:%v", clusterName, address, port)
	p.clusterName = clusterName
	p.address = address
	p.memberPort = port
	p.knownKinds = knownKinds
	p.statusValue = statusValue
	p.statusValueSerializer = serializer
	p.deregistered = false
	p.shutdown = false

	p.UpdateTTL()
	return nil
}

// DeregisterMember set the shutdown to true preventing any more TTL updates
func (p *AutoManagedProvider) DeregisterMember() error {
	deregisteredMutex.Lock()
	defer deregisteredMutex.Unlock()

	p.deregistered = true
	return nil
}

// Shutdown set the shutdown to true preventing any more TTL updates
func (p *AutoManagedProvider) Shutdown() error {
	shutdownMutex.Lock()
	defer shutdownMutex.Unlock()

	p.shutdown = true
	p.activeProvider.Close()
	return nil
}

// UpdateTTL sets up an endpoint to respond to other members
func (p *AutoManagedProvider) UpdateTTL() {
	activeProviderRunningMutex.Lock()
	running := p.activeProviderRunning
	activeProviderRunningMutex.Unlock()

	if (p.isShutdown() || p.isDeregistered()) && running {
		p.activeProvider.Close()
		return
	}

	if running {
		return
	}

	// its not running and its not shutdown or deregistered
	// its also not a test (this should be refactored)

	if !p.activeProviderTesting {
		p.activeProvider = echo.New()
		p.activeProvider.HideBanner = true
		p.activeProvider.GET("/_health", func(context echo.Context) error {
			return context.JSON(http.StatusOK, p.getCurrentNode())
		})
	}

	go func() {

		activeProviderRunningMutex.Lock()
		p.activeProviderRunning = true
		activeProviderRunningMutex.Unlock()

		appURI := fmt.Sprintf("0.0.0.0:%d", p.autoManagePort)
		plog.Error("Automanaged server stopping..!", log.Error(p.activeProvider.Start(appURI)))

		activeProviderRunningMutex.Lock()
		p.activeProviderRunning = false
		activeProviderRunningMutex.Unlock()
	}()
}

func (p *AutoManagedProvider) UpdateMemberStatusValue(statusValue cluster.MemberStatusValue) error {
	p.statusValue = statusValue
	if p.statusValue == nil {
		return nil
	}
	return nil
}

// MonitorMemberStatusChanges creates a go routine that continuously checks other members
func (p *AutoManagedProvider) MonitorMemberStatusChanges() {
	if !p.monitoringStatus {
		go func() {
			for !p.isShutdown() && !p.isDeregistered() {
				p.monitorStatuses()
			}
		}()
	}
	p.monitoringStatus = true
}

// GetHealthStatus returns an error if the cluster health status has problems
func (p *AutoManagedProvider) GetHealthStatus() error {
	var err error
	clusterTTLErrorMutex.Lock()
	clusterMonitorErrorMutex.Lock()
	defer clusterMonitorErrorMutex.Unlock()
	defer clusterTTLErrorMutex.Unlock()

	if p.clusterTTLError != nil {
		err = fmt.Errorf("TTL: %s", p.clusterTTLError.Error())
	}

	if p.clusterMonitorError != nil {
		if err != nil {
			err = fmt.Errorf("%s - Monitor: %s", err.Error(), p.clusterMonitorError.Error())
		}
		err = fmt.Errorf("Monitor: %s", p.clusterMonitorError.Error())
	}

	return err
}

//
// Private methods
//

func (p *AutoManagedProvider) isShutdown() bool {
	shutdownMutex.Lock()
	defer shutdownMutex.Unlock()
	return p.shutdown
}

func (p *AutoManagedProvider) isDeregistered() bool {
	deregisteredMutex.Lock()
	defer deregisteredMutex.Unlock()
	return p.deregistered
}

func (p *AutoManagedProvider) isActiveProviderRunning() bool {
	activeProviderRunningMutex.Lock()
	defer activeProviderRunningMutex.Unlock()
	return p.activeProviderRunning
}

// monitorStatuses checks for node changes in the cluster
func (p *AutoManagedProvider) monitorStatuses() {
	clusterMonitorErrorMutex.Lock()
	defer clusterMonitorErrorMutex.Unlock()

	autoManagedNodes, err := p.checkNodes()
	if err != nil && len(autoManagedNodes) == 0 {
		plog.Error("Failure reaching nodes", log.Error(err))
		p.clusterMonitorError = err
		time.Sleep(p.refreshTTL)
		return
	}

	p.knownNodes = autoManagedNodes

	// we should probably check if the cluster needs to be updated..
	res := make(cluster.ClusterTopologyEvent, len(p.knownNodes))
	for idx, node := range p.knownNodes {
		if node == nil {
			continue
		}
		key := fmt.Sprintf("%v/%v:%v", p.clusterName, node.Address, node.Port)
		memberID := key
		memberStatusVal := p.statusValueSerializer.FromValueBytes([]byte(key))
		ms := &cluster.MemberStatus{
			MemberID:    memberID,
			Host:        node.Address,
			Port:        node.Port,
			Kinds:       node.Kinds,
			Alive:       true,
			StatusValue: memberStatusVal,
		}
		res[idx] = ms
	}
	p.clusterMonitorError = nil
	// publish the current cluster topology onto the event stream
	eventstream.Publish(res)
	time.Sleep(p.refreshTTL)

}

// checkNodes pings all the nodes and returns the new cluster topology
func (p *AutoManagedProvider) checkNodes() ([]*NodeModel, error) {

	allNodes := make([]*NodeModel, len(p.hosts))
	g, _ := errgroup.WithContext(context.Background())

	for indice, nodeHost := range p.hosts {

		idx, el := indice, nodeHost // https://golang.org/doc/faq#closures_and_goroutines

		// Calling go funcs to execute the node check
		g.Go(func() error {

			url := fmt.Sprintf("http://%s/_health", el)
			req, err := http.NewRequest("GET", url, nil)
			if err != nil {
				plog.Error("Couldnt fetch node health status", log.Error(err))
				return err
			}

			resp, err := p.httpClient.Do(req)
			if err != nil {
				return err
			}

			defer resp.Body.Close() // nolint: errcheck

			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("non 200 status returned: %d - from node: %s", resp.StatusCode, el)
			}

			var node *NodeModel
			err = json.NewDecoder(resp.Body).Decode(&node)
			if err != nil {
				return fmt.Errorf("could not deserialize response: %v - from node: %s", resp, el)
			}

			allNodes[idx] = node
			return nil
		})
	}

	// waits until all functions have returned
	err := g.Wait()
	retNodes := []*NodeModel{}

	// clear out the nil ones
	for _, node := range allNodes {
		if node != nil {
			retNodes = append(retNodes, node)
		}
	}

	return retNodes, err
}

func (p *AutoManagedProvider) deregisterService() {
	deregisteredMutex.Lock()
	defer deregisteredMutex.Unlock()

	p.deregistered = true
}

func (p *AutoManagedProvider) startActiveProvider() {
	activeProviderRunningMutex.Lock()
	running := p.activeProviderRunning
	activeProviderRunningMutex.Unlock()

	if !running {

		if !p.activeProviderTesting {
			p.activeProvider = echo.New()
			p.activeProvider.HideBanner = true
			p.activeProvider.GET("/_health", func(context echo.Context) error {
				return context.JSON(http.StatusOK, p.getCurrentNode())
			})
		}

		appURI := fmt.Sprintf("0.0.0.0:%d", p.autoManagePort)

		go func() {

			activeProviderRunningMutex.Lock()
			p.activeProviderRunning = true
			activeProviderRunningMutex.Unlock()

			plog.Error("Automanaged server stopping..!", log.Error(p.activeProvider.Start(appURI)))

			activeProviderRunningMutex.Lock()
			p.activeProviderRunning = false
			activeProviderRunningMutex.Unlock()
		}()
	}
}

func (p *AutoManagedProvider) stopActiveProvider() {
	p.activeProvider.Close()

}

func (p *AutoManagedProvider) getCurrentNode() *NodeModel {
	return NewNode(p.clusterName, p.address, p.memberPort, p.autoManagePort, p.knownKinds)
}
