package automanaged

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/net/context"

	"github.com/asynkron/protoactor-go/cluster"
	"github.com/labstack/echo"
	"golang.org/x/sync/errgroup"
)

// TODO: needs to be attached to the provider instance
var (
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
	clusterName           string
	address               string
	autoManagePort        int
	memberPort            int
	knownKinds            []string
	knownNodes            []*NodeModel
	hosts                 []string
	refreshTTL            time.Duration
	clusterTTLError       error
	clusterMonitorError   error
	cluster               *cluster.Cluster
}

// New creates a AutoManagedProvider that connects locally
func New() *AutoManagedProvider {
	return NewWithConfig(
		2*time.Second,
		6330,
		"localhost:6330",
	)
}

// NewWithConfig creates an Automanaged Provider that connects to an all the hosts
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

func (p *AutoManagedProvider) init(cluster *cluster.Cluster) error {
	host, port, err := cluster.ActorSystem.GetHostPort()
	if err != nil {
		return err
	}

	p.clusterName = cluster.Config.Name
	p.address = host
	p.memberPort = port
	p.knownKinds = cluster.GetClusterKinds()
	p.deregistered = false
	p.shutdown = false
	p.cluster = cluster
	return nil
}

func (p *AutoManagedProvider) StartMember(cluster *cluster.Cluster) error {
	if err := p.init(cluster); err != nil {
		return err
	}
	p.UpdateTTL()
	p.monitorMemberStatusChanges()
	return nil
}

func (p *AutoManagedProvider) StartClient(cluster *cluster.Cluster) error {
	if err := p.init(cluster); err != nil {
		return err
	}
	// p.UpdateTTL()
	p.monitorMemberStatusChanges()
	return nil
}

// DeregisterMember set the shutdown to true preventing anymore TTL updates
func (p *AutoManagedProvider) DeregisterMember() error {
	deregisteredMutex.Lock()
	defer deregisteredMutex.Unlock()

	p.deregistered = true
	return nil
}

// Shutdown set the shutdown to true preventing anymore TTL updates
func (p *AutoManagedProvider) Shutdown(graceful bool) error {
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

	// it's not running, and it's not shutdown or de-registered
	// it's also not a test (this should be refactored)

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
		p.cluster.Logger().Error("Automanaged server stopping..!", slog.Any("error", p.activeProvider.Start(appURI)))

		activeProviderRunningMutex.Lock()
		p.activeProviderRunning = false
		activeProviderRunningMutex.Unlock()
	}()
}

// MonitorMemberStatusChanges creates a go routine that continuously checks other members
func (p *AutoManagedProvider) monitorMemberStatusChanges() {
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
		} else {
			err = fmt.Errorf("monitor: %s", p.clusterMonitorError.Error())
		}
	}

	return err
}

//
// Private methods
//

// monitorStatuses checks for node changes in the cluster
func (p *AutoManagedProvider) monitorStatuses() {
	clusterMonitorErrorMutex.Lock()
	defer clusterMonitorErrorMutex.Unlock()

	autoManagedNodes, err := p.checkNodes()
	if err != nil && len(autoManagedNodes) == 0 {
		p.cluster.Logger().Error("Failure reaching nodes", slog.Any("error", err))
		p.clusterMonitorError = err
		time.Sleep(p.refreshTTL)
		return
	}
	// we should probably check if the cluster needs to be updated.
	var members []*cluster.Member
	var newNodes []*NodeModel
	for _, node := range autoManagedNodes {
		if node == nil || node.ClusterName != p.clusterName {
			continue
		}
		ms := &cluster.Member{
			Id:    node.ID,
			Host:  node.Address,
			Port:  int32(node.Port),
			Kinds: node.Kinds,
		}
		members = append(members, ms)
		newNodes = append(newNodes, node)
	}

	p.knownNodes = newNodes
	p.clusterMonitorError = nil
	// publish the current cluster topology onto the event stream
	p.cluster.MemberList.UpdateClusterTopology(members)
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
				p.cluster.Logger().Error("Couldn't request node health status", slog.Any("error", err), slog.String("autoManMemberUrl", url))
				return err
			}

			resp, err := p.httpClient.Do(req)
			if err != nil {
				p.cluster.Logger().Error("Bad connection to the node health status", slog.Any("error", err), slog.String("autoManMemberUrl", url))
				return err
			}

			defer resp.Body.Close() // nolint: errcheck

			if resp.StatusCode != http.StatusOK {
				err = fmt.Errorf("non 200 status returned: %d - from node: %s", resp.StatusCode, el)
				p.cluster.Logger().Error("Bad response from the node health status", slog.Any("error", err), slog.String("autoManMemberUrl", url))
				return err
			}

			var node *NodeModel
			err = json.NewDecoder(resp.Body).Decode(&node)
			if err != nil {
				err = fmt.Errorf("could not deserialize response: %v - from node: %s", resp, el)
				p.cluster.Logger().Error("Bad data from the node health status", slog.Any("error", err), slog.String("autoManMemberUrl", url))
				return err
			}

			allNodes[idx] = node
			return nil
		})
	}

	// waits until all functions have returned
	err := g.Wait()
	var retNodes []*NodeModel

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
			err := p.activeProvider.Start(appURI)
			p.cluster.Logger().Error("Automanaged server stopping..!", slog.Any("error", err))

			activeProviderRunningMutex.Lock()
			p.activeProviderRunning = false
			activeProviderRunningMutex.Unlock()
		}()
	}
}

func (p *AutoManagedProvider) stopActiveProvider() {
	p.activeProvider.Close()
}

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

func (p *AutoManagedProvider) getCurrentNode() *NodeModel {
	return NewNode(p.clusterName, p.cluster.ActorSystem.ID, p.address, p.memberPort, p.autoManagePort, p.knownKinds)
}
