package gocbcore

import (
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

// This class represents the base client handling connections to a Couchbase Server.
// This is used internally by the higher level classes for communicating with the cluster,
// it can also be used to perform more advanced operations with a cluster.
type Agent struct {
	bucket            string
	password          string
	tlsConfig         *tls.Config
	initFn            memdInitFunc
	useMutationTokens bool

	routingInfo routeDataPtr
	numVbuckets int

	serverFailuresLock sync.Mutex
	serverFailures     map[string]time.Time

	httpCli *http.Client

	serverConnectTimeout time.Duration
	serverWaitTimeout    time.Duration
	nmvRetryDelay        time.Duration

	shutdownWaitCh chan *memdPipeline
}

// The timeout for each server connection, including all authentication steps.
func (c *Agent) ServerConnectTimeout() time.Duration {
	return c.serverConnectTimeout
}

// Sets the timeout for each server connection.
func (c *Agent) SetServerConnectTimeout(timeout time.Duration) {
	c.serverConnectTimeout = timeout
}

// Returns a pre-configured HTTP Client for communicating with
// Couchbase Server.  You must still specify authentication
// information for any dispatched requests.
func (c *Agent) HttpClient() *http.Client {
	return c.httpCli
}

type AuthFunc func(client AuthClient, deadline time.Time) error

type AgentConfig struct {
	MemdAddrs         []string
	HttpAddrs         []string
	TlsConfig         *tls.Config
	BucketName        string
	Password          string
	AuthHandler       AuthFunc
	UseMutationTokens bool

	ConnectTimeout       time.Duration
	ServerConnectTimeout time.Duration
	NmvRetryDelay        time.Duration
}

// Creates an agent for performing normal operations.
func CreateAgent(config *AgentConfig) (*Agent, error) {
	initFn := func(pipeline *memdPipeline, deadline time.Time) error {
		return config.AuthHandler(&authClient{pipeline}, deadline)
	}
	return createAgent(config, initFn)
}

// **INTERNAL**
// Creates an agent for performing DCP operations.
func CreateDcpAgent(config *AgentConfig, dcpStreamName string) (*Agent, error) {
	// We wrap the authorization system to force DCP channel opening
	//   as part of the "initialization" for any servers.
	dcpInitFn := func(pipeline *memdPipeline, deadline time.Time) error {
		if err := config.AuthHandler(&authClient{pipeline}, deadline); err != nil {
			return err
		}
		return doOpenDcpChannel(pipeline, dcpStreamName, deadline)
	}
	return createAgent(config, dcpInitFn)
}

func createAgent(config *AgentConfig, initFn memdInitFunc) (*Agent, error) {
	c := &Agent{
		bucket:    config.BucketName,
		password:  config.Password,
		tlsConfig: config.TlsConfig,
		initFn:    initFn,
		httpCli: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: config.TlsConfig,
			},
		},
		useMutationTokens:    config.UseMutationTokens,
		serverFailures:       make(map[string]time.Time),
		serverConnectTimeout: config.ServerConnectTimeout,
		serverWaitTimeout:    5 * time.Second,
		nmvRetryDelay:        config.NmvRetryDelay,
	}

	deadline := time.Now().Add(config.ConnectTimeout)
	if err := c.connect(config.MemdAddrs, config.HttpAddrs, deadline); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Agent) cccpLooper() {
	tickTime := time.Second * 10
	maxWaitTime := time.Second * 3

	logDebugf("CCCP Looper starting.")

	for {
		// Wait 10 seconds
		time.Sleep(tickTime)

		routingInfo := c.routingInfo.get()
		if routingInfo == nil {
			// If we have a blank routingInfo, it indicates the client is shut down.
			break
		}

		numServers := len(routingInfo.servers)
		if numServers == 0 {
			logDebugf("CCCPPOLL: No servers")
			continue
		}

		srvIdx := rand.Intn(numServers)
		srv := routingInfo.servers[srvIdx]

		// Force config refresh from random node
		cccpBytes, err := doCccpRequest(srv, time.Now().Add(maxWaitTime))
		if err != nil {
			logDebugf("CCCPPOLL: Failed to retrieve CCCP config. %v", err)
			continue
		}

		bk, err := parseConfig(cccpBytes, srv.Hostname())
		if err != nil {
			logDebugf("CCCPPOLL: Failed to parse CCCP config. %v", err)
			continue
		}

		logDebugf("CCCPPOLL: Received new config")
		c.updateConfig(bk)
	}
}

func (c *Agent) connect(memdAddrs, httpAddrs []string, deadline time.Time) error {
	logDebugf("Attempting to connect...")

	for _, thisHostPort := range memdAddrs {
		logDebugf("Trying server at %s", thisHostPort)

		srvDeadlineTm := time.Now().Add(c.serverConnectTimeout)
		if srvDeadlineTm.After(deadline) {
			srvDeadlineTm = deadline
		}

		srv := CreateMemdPipeline(thisHostPort)

		logDebugf("Trying to connect")
		err := c.connectPipeline(srv, srvDeadlineTm)
		if err != nil {
			if err == ErrAuthError {
				return err
			}
			logDebugf("Connecting failed! %v", err)
			continue
		}

		logDebugf("Attempting to request CCCP configuration")
		cccpBytes, err := doCccpRequest(srv, srvDeadlineTm)
		if err != nil {
			logDebugf("Failed to retrieve CCCP config. %v", err)
			srv.Close()
			continue
		}

		bk, err := parseConfig(cccpBytes, srv.Hostname())
		if err != nil {
			srv.Close()
			continue
		}

		if !bk.supportsCccp() {
			// No CCCP support, fall back to HTTP!
			srv.Close()
			break
		}

		routeCfg := buildRouteConfig(bk, c.IsSecure())
		if !routeCfg.IsValid() {
			// Something is invalid about this config, keep trying
			srv.Close()
			continue
		}

		logDebugf("Successfully connected")

		// Build some fake routing data, this is used to essentially 'pass' the
		//   server connection we already have over to the config update function.
		c.routingInfo.update(nil, &routeData{
			servers: []*memdPipeline{srv},
		})

		c.numVbuckets = len(routeCfg.vbMap)
		c.applyConfig(routeCfg)

		srv.SetHandlers(c.handleServerNmv, c.handleServerDeath)

		go c.cccpLooper()

		return nil
	}

	signal := make(chan error, 1)

	var epList []string
	for _, hostPort := range httpAddrs {
		if !c.IsSecure() {
			epList = append(epList, fmt.Sprintf("http://%s", hostPort))
		} else {
			epList = append(epList, fmt.Sprintf("https://%s", hostPort))
		}
	}
	c.routingInfo.update(nil, &routeData{
		mgmtEpList: epList,
	})

	var routeCfg *routeConfig

	logDebugf("Starting HTTP looper! %v", epList)
	go c.httpLooper(func(cfg *cfgBucket, err error) bool {
		if err != nil {
			signal <- err
			return true
		}

		newRouteCfg := buildRouteConfig(cfg, c.IsSecure())
		if !newRouteCfg.IsValid() {
			// Something is invalid about this config, keep trying
			return false
		}

		routeCfg = newRouteCfg
		signal <- nil
		return true
	})

	err := <-signal
	if err != nil {
		return err
	}

	c.numVbuckets = len(routeCfg.vbMap)
	c.applyConfig(routeCfg)

	return nil
}

// Shuts down the agent, disconnecting from all servers and failing
// any outstanding operations with ErrShutdown.
func (agent *Agent) Close() {
	// Set up a channel so we can find out when servers shut down.
	agent.shutdownWaitCh = make(chan *memdPipeline)

	// Clear the routingInfo so no new operations are performed
	//   and retrieve the last active routing configuration
	routingInfo := agent.routingInfo.clear()
	if routingInfo == nil {
		return
	}

	// Loop all the currently running servers and close their
	//   connections.  Their requests will be drained below.
	for _, s := range routingInfo.servers {
		s.Close()
	}

	// Clear any extraneous queues that may still contain
	//   requests which are not pending on a server queue.
	if routingInfo.deadQueue != nil {
		routingInfo.deadQueue.Drain(func(req *memdQRequest) {
			req.Callback(nil, nil, ErrShutdown)
		}, nil)
	}
	if routingInfo.waitQueue != nil {
		routingInfo.waitQueue.Drain(func(req *memdQRequest) {
			req.Callback(nil, nil, ErrShutdown)
		}, nil)
	}

	// Loop all the currently running servers and wait for them
	//   to stop running then drain their requests as errors
	//   (this also closes the server conn).
	for range routingInfo.servers {
		s := <-agent.shutdownWaitCh
		s.Drain(func(req *memdQRequest) {
			req.Callback(nil, nil, ErrShutdown)
		})
	}
}

// Returns whether this client is connected via SSL.
func (c *Agent) IsSecure() bool {
	return c.tlsConfig != nil
}

// Translates a particular key to its assigned vbucket.
func (c *Agent) KeyToVbucket(key []byte) uint16 {
	if c.NumVbuckets() <= 0 {
		return 0xFFFF
	}
	return uint16(cbCrc(key) % uint32(c.NumVbuckets()))
}

// Returns the number of VBuckets configured on the
// connected cluster.
func (c *Agent) NumVbuckets() int {
	return c.numVbuckets
}

// Returns the number of replicas configured on the
// connected cluster.
func (c *Agent) NumReplicas() int {
	routingInfo := c.routingInfo.get()
	if routingInfo == nil {
		return 0
	}
	return len(routingInfo.vbMap[0]) - 1
}

// Returns number of servers accessible for K/V.
func (c *Agent) NumServers() int {
	routingInfo := c.routingInfo.get()
	if routingInfo == nil {
		return 0
	}
	return len(routingInfo.queues)
}

// Returns list of VBuckets on the server.
func (c *Agent) VbucketsOnServer(index int) []uint16 {
	var vbuckets []uint16
	routingInfo := c.routingInfo.get()
	if routingInfo == nil {
		return vbuckets
	}

	for vb, entry := range routingInfo.vbMap {
		if entry[0] == index {
			vbuckets = append(vbuckets, uint16(vb))
		}
	}
	return vbuckets
}

// Returns all the available endpoints for performing
// map-reduce queries.
func (agent *Agent) CapiEps() []string {
	routingInfo := agent.routingInfo.get()
	if routingInfo == nil {
		return nil
	}
	return routingInfo.capiEpList
}

// Returns all the available endpoints for performing
// management queries.
func (agent *Agent) MgmtEps() []string {
	routingInfo := agent.routingInfo.get()
	if routingInfo == nil {
		return nil
	}
	return routingInfo.mgmtEpList
}

// Returns all the available endpoints for performing
// N1QL queries.
func (agent *Agent) N1qlEps() []string {
	routingInfo := agent.routingInfo.get()
	if routingInfo == nil {
		return nil
	}
	return routingInfo.n1qlEpList
}

// Returns all the available endpoints for performing
// FTS queries.
func (agent *Agent) FtsEps() []string {
	routingInfo := agent.routingInfo.get()
	if routingInfo == nil {
		return nil
	}
	return routingInfo.ftsEpList
}

func doCccpRequest(pipeline *memdPipeline, deadline time.Time) ([]byte, error) {
	resp, err := pipeline.ExecuteRequest(&memdQRequest{
		memdRequest: memdRequest{
			Magic:    ReqMagic,
			Opcode:   CmdGetClusterConfig,
			Datatype: 0,
			Cas:      0,
			Extras:   nil,
			Key:      nil,
			Value:    nil,
		},
	}, deadline)
	if err != nil {
		return nil, err
	}

	return resp.Value, nil
}

func doOpenDcpChannel(pipeline *memdPipeline, streamName string, deadline time.Time) error {
	extraBuf := make([]byte, 8)
	binary.BigEndian.PutUint32(extraBuf[0:], 0)
	binary.BigEndian.PutUint32(extraBuf[4:], 1)

	_, err := pipeline.ExecuteRequest(&memdQRequest{
		memdRequest: memdRequest{
			Magic:    ReqMagic,
			Opcode:   CmdDcpOpenConnection,
			Datatype: 0,
			Cas:      0,
			Extras:   extraBuf,
			Key:      []byte(streamName),
			Value:    nil,
		},
	}, deadline)
	if err != nil {
		return err
	}

	return nil
}
