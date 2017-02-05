package gocbcore

import (
	"crypto/tls"
	"encoding/binary"
	"net"
	"time"
)

func (c *Agent) waitAndRetryOperation(req *memdQRequest) {
	if c.nmvRetryDelay == 0 {
		c.redispatchDirect(req)
	} else {
		time.AfterFunc(c.nmvRetryDelay, func() {
			c.redispatchDirect(req)
		})
	}
}

func (c *Agent) handleServerNmv(s *memdPipeline, req *memdQRequest, resp *memdResponse) {
	// Try to parse the value as a bucket configuration
	bk, err := parseConfig(resp.Value, s.Hostname())
	if err == nil {
		c.updateConfig(bk)
	}

	// Redirect it!  This may actually come back to this server, but I won't tell
	//   if you don't ;)
	c.waitAndRetryOperation(req)
}

func (c *Agent) handleServerDeath(s *memdPipeline) {
	// Check if we are shutting down, if so, simply notify the shutdown
	//   method that we are going away.
	routeData := c.routingInfo.get()
	if routeData == nil {
		c.shutdownWaitCh <- s
		return
	}

	// Refresh the routing data with the existing configuration, this has
	//   the effect of attempting to rebuild the dead server.
	c.updateConfig(nil)

	// TODO(brett19): We probably should actually try other ways of resolving
	//  the issue, like requesting a new configuration.
}

func appendFeatureCode(bytes []byte, feature HelloFeature) []byte {
	bytes = append(bytes, 0, 0)
	binary.BigEndian.PutUint16(bytes[len(bytes)-2:], uint16(FeatureSeqNo))
	return bytes
}

func (c *Agent) tryHello(pipeline *memdPipeline, deadline time.Time) error {
	var featuresBytes []byte

	if c.useMutationTokens {
		featuresBytes = appendFeatureCode(featuresBytes, FeatureSeqNo)
	}

	_, err := pipeline.ExecuteRequest(&memdQRequest{
		memdRequest: memdRequest{
			Magic:  ReqMagic,
			Opcode: CmdHello,
			Key:    []byte("gocb/" + GoCbCoreVersionStr),
			Value:  featuresBytes,
		},
	}, deadline)

	return err
}

// Attempt to connect a server, this function must be called
//  in its own goroutine and will ensure that offline servers
//  are not spammed with connection attempts.
func (agent *Agent) connectServer(server *memdPipeline) {
	agent.serverFailuresLock.Lock()
	failureTime := agent.serverFailures[server.address]
	agent.serverFailuresLock.Unlock()

	if !failureTime.IsZero() {
		waitedTime := time.Since(failureTime)
		if waitedTime < agent.serverWaitTimeout {
			time.Sleep(agent.serverWaitTimeout - waitedTime)

			if !agent.checkPendingServer(server) {
				// Server is no longer pending.  Stop trying.
				return
			}
		}
	}

	err := agent.connectPipeline(server, time.Now().Add(agent.serverConnectTimeout))
	if err != nil {
		agent.serverFailuresLock.Lock()
		agent.serverFailures[server.address] = time.Now()
		agent.serverFailuresLock.Unlock()

		// Force a config update which will clear away this dead pending
		// server and create a new one to connect with.
		agent.updateConfig(nil)
		return
	}

	if !agent.activatePendingServer(server) {
		// If this is no longer a valid pending server, we should shut
		//   it down!
		server.Close()
	}
}

func (c *Agent) connectPipeline(pipeline *memdPipeline, deadline time.Time) error {
	logDebugf("Attempting to connect pipeline to %s", pipeline.address)

	// Copy the tls configuration since we need to provide the hostname for each
	// server that we connect to so that the certificate can be validated properly.
	var tlsConfig *tls.Config = nil
	if c.tlsConfig != nil {
		host, _, _ := net.SplitHostPort(pipeline.address)
		tlsConfig = &tls.Config{
			InsecureSkipVerify: c.tlsConfig.InsecureSkipVerify,
			RootCAs:            c.tlsConfig.RootCAs,
			ServerName:         host,
		}
	}

	memdConn, err := DialMemdConn(pipeline.address, tlsConfig, deadline)
	if err != nil {
		logDebugf("Failed to connect. %v", err)
		pipeline.lock.Lock()
		pipeline.isClosed = true
		pipeline.lock.Unlock()
		return err
	}

	logDebugf("Connected.")
	pipeline.conn = memdConn
	go pipeline.Run()

	logDebugf("Authenticating...")
	err = c.initFn(pipeline, deadline)
	if err != nil {
		logDebugf("Failed to authenticate. %v", err)
		memdConn.Close()
		return err
	}

	c.tryHello(pipeline, deadline)

	return nil
}

// Drains all the requests out of the queue for this server.  This must be
//   invoked only once this server no longer exists in the routing data or an
//   infinite loop will likely occur.
func (c *Agent) shutdownPipeline(s *memdPipeline) {
	s.Drain(func(req *memdQRequest) {
		c.redispatchDirect(req)
	})

	s.Close()
}

func (agent *Agent) checkPendingServer(server *memdPipeline) bool {
	oldRouting := agent.routingInfo.get()
	if oldRouting == nil {
		return false
	}

	// Find the index of the pending server we want to swap
	var serverIdx int = -1
	for i, s := range oldRouting.pendingServers {
		if s == server {
			serverIdx = i
			break
		}
	}

	return serverIdx != -1
}

func (agent *Agent) activatePendingServer(server *memdPipeline) bool {
	logDebugf("Activating Server...")

	var oldRouting *routeData
	for {
		oldRouting = agent.routingInfo.get()
		if oldRouting == nil {
			return false
		}

		// Find the index of the pending server we want to swap
		var serverIdx int = -1
		for i, s := range oldRouting.pendingServers {
			if s == server {
				serverIdx = i
				break
			}
		}

		if serverIdx == -1 {
			// This server is no longer in the list
			return false
		}

		var newRouting *routeData = &routeData{
			revId:      oldRouting.revId,
			capiEpList: oldRouting.capiEpList,
			mgmtEpList: oldRouting.mgmtEpList,
			n1qlEpList: oldRouting.n1qlEpList,
			ftsEpList:  oldRouting.ftsEpList,
			vbMap:      oldRouting.vbMap,
			source:     oldRouting.source,
			deadQueue:  oldRouting.deadQueue,
			bktType:    oldRouting.bktType,
		}

		// Copy the lists
		newRouting.queues = append(newRouting.queues, oldRouting.queues...)
		newRouting.servers = append(newRouting.servers, oldRouting.servers...)
		newRouting.pendingServers = append(newRouting.pendingServers, oldRouting.pendingServers...)

		// Swap around the pending server to being an active one
		newRouting.servers = append(newRouting.servers, server)
		newRouting.queues[serverIdx] = server.queue
		newRouting.pendingServers[serverIdx] = nil

		// Update to the new waitQueue
		for i, q := range newRouting.queues {
			if q == oldRouting.waitQueue {
				if newRouting.waitQueue == nil {
					newRouting.waitQueue = createMemdQueue()
				}
				newRouting.queues[i] = newRouting.waitQueue
			}
		}

		// Double-check the queue to make sure its still big enough.
		if len(newRouting.queues) != len(oldRouting.queues) {
			panic("Pending server swap corrupted the queue list.")
		}

		// Attempt to atomically update the routing data
		if !agent.routingInfo.update(oldRouting, newRouting) {
			// Someone preempted us, let's restart and try again...
			continue
		}

		server.SetHandlers(agent.handleServerNmv, agent.handleServerDeath)

		logDebugf("Switching routing data (server activation %d)...", serverIdx)
		oldRouting.logDebug()
		logDebugf("To new data...")
		newRouting.logDebug()

		// We've successfully swapped to the new config, lets finish building the
		//   new routing data's connections and destroy/draining old connections.
		break
	}

	oldWaitQueue := oldRouting.waitQueue
	if oldWaitQueue != nil {
		oldWaitQueue.Drain(func(req *memdQRequest) {
			agent.redispatchDirect(req)
		}, nil)
	}

	return true
}

// Accepts a cfgBucket object representing a cluster configuration and rebuilds the server list
//  along with any routing information for the Client.  Passing no config will refresh the existing one.
//  This method MUST NEVER BLOCK due to its use from various contention points.
func (agent *Agent) applyConfig(cfg *routeConfig) {
	// Check some basic things to ensure consistency!
	if len(cfg.vbMap) != agent.numVbuckets {
		panic("Received a configuration with a different number of vbuckets.")
	}

	var oldRouting *routeData
	var newRouting *routeData = &routeData{
		revId:      cfg.revId,
		capiEpList: cfg.capiEpList,
		mgmtEpList: cfg.mgmtEpList,
		n1qlEpList: cfg.n1qlEpList,
		ftsEpList:  cfg.ftsEpList,
		vbMap:      cfg.vbMap,
		bktType:    cfg.bktType,
		source:     cfg,
	}

	var needsDeadQueue bool
	for _, replicaList := range cfg.vbMap {
		for _, srvIdx := range replicaList {
			if srvIdx == -1 {
				needsDeadQueue = true
				break
			}
		}
	}
	if needsDeadQueue {
		newRouting.deadQueue = createMemdQueue()
	}

	var createdServers []*memdPipeline
	for {
		oldRouting = agent.routingInfo.get()
		if oldRouting == nil {
			return
		}

		if newRouting.revId < oldRouting.revId {
			logDebugf("Ignoring new configuration as it has an older revision id.")
			return
		}

		// Reset the lists before each iteration
		newRouting.queues = nil
		newRouting.servers = nil
		newRouting.pendingServers = nil

		for _, hostPort := range cfg.kvServerList {
			var thisServer *memdPipeline

			// See if this server exists in the old routing data and is still alive
			for _, oldServer := range oldRouting.servers {
				if oldServer.Address() == hostPort && !oldServer.IsClosed() {
					thisServer = oldServer
					break
				}
			}

			// If we found a still active connection in our old routing table,
			//   we just need to copy it over to the new routing table.
			if thisServer != nil {
				newRouting.pendingServers = append(newRouting.pendingServers, nil)
				newRouting.servers = append(newRouting.servers, thisServer)
				newRouting.queues = append(newRouting.queues, thisServer.queue)
				continue
			}

			// Search for any servers we are already trying to connect with.
			for _, oldServer := range oldRouting.pendingServers {
				if oldServer != nil && oldServer.Address() == hostPort && !oldServer.IsClosed() {
					thisServer = oldServer
					break
				}
			}

			// If we do not already have a pending server we are trying to
			//   connect to, we should build one!
			if thisServer == nil {
				thisServer = CreateMemdPipeline(hostPort)
				createdServers = append(createdServers, thisServer)
			}

			if newRouting.waitQueue == nil {
				newRouting.waitQueue = createMemdQueue()
			}

			newRouting.pendingServers = append(newRouting.pendingServers, thisServer)
			newRouting.queues = append(newRouting.queues, newRouting.waitQueue)
		}

		// Check everything is sane
		if len(newRouting.queues) < len(cfg.kvServerList) {
			panic("Failed to generate a queues list that matches the config server list.")
		}

		// Attempt to atomically update the routing data
		if !agent.routingInfo.update(oldRouting, newRouting) {
			// Someone preempted us, let's restart and try again...
			continue
		}

		// We've successfully swapped to the new config, lets finish building the
		//   new routing data's connections and destroy/draining old connections.
		break
	}

	logDebugf("Switching routing data (update)...")
	oldRouting.logDebug()
	logDebugf("To new data...")
	newRouting.logDebug()

	// Launch all the new servers
	for _, newServer := range createdServers {
		go agent.connectServer(newServer)
	}

	// Identify all the dead servers and drain their requests
	for _, oldServer := range oldRouting.servers {
		found := false
		for _, newServer := range newRouting.servers {
			if newServer == oldServer {
				found = true
				break
			}
		}
		if !found {
			go agent.shutdownPipeline(oldServer)
		}
	}

	oldWaitQueue := oldRouting.waitQueue
	if oldWaitQueue != nil {
		oldWaitQueue.Drain(func(req *memdQRequest) {
			agent.redispatchDirect(req)
		}, nil)
	}

	oldDeadQueue := oldRouting.deadQueue
	if oldDeadQueue != nil {
		oldDeadQueue.Drain(func(req *memdQRequest) {
			agent.redispatchDirect(req)
		}, nil)
	}
}

func (agent *Agent) updateConfig(bk *cfgBucket) {
	if bk == nil {
		// Use the existing config if none was passed.
		oldRouting := agent.routingInfo.get()
		if oldRouting == nil {
			// If there is no previous config, we can't do anything
			return
		}

		agent.applyConfig(oldRouting.source)
	} else {
		// Normalize the cfgBucket to a routeConfig and apply it.
		routeCfg := buildRouteConfig(bk, agent.IsSecure())
		if !routeCfg.IsValid() {
			// We received an invalid configuration, lets shutdown.
			agent.Close()
			return
		}

		agent.applyConfig(routeCfg)
	}
}

func (agent *Agent) routeRequest(req *memdQRequest) (*memdQueue, error) {
	routingInfo := agent.routingInfo.get()
	if routingInfo == nil {
		return nil, nil
	}

	var srvIdx int
	repId := req.ReplicaIdx

	// Route to specific server
	if repId < 0 {
		srvIdx = -repId - 1
		if srvIdx >= len(routingInfo.queues) {
			return nil, ErrInvalidServer
		}
		return routingInfo.queues[srvIdx], nil
	}

	if routingInfo.bktType == BktTypeCouchbase {
		// Targeting a specific replica; repId >= 0
		if repId >= routingInfo.source.numReplicas+1 {
			return nil, ErrInvalidReplica
		}

		if req.Key != nil {
			srvIdx, req.Vbucket = routingInfo.MapKeyVBucket(req.Key, repId)
		} else {
			// Filter explicit vBucket input. Really only used in OBSERVE
			if int(req.Vbucket) >= len(routingInfo.vbMap) {
				return nil, ErrInvalidVBucket
			}
			srvIdx = routingInfo.vbMap[req.Vbucket][repId]
		}
	} else if routingInfo.bktType == BktTypeMemcached {
		if repId > 0 {
			// Error. Memcached buckets don't understand replicas!
			return nil, ErrInvalidReplica
		}

		if req.Key == nil {
			panic("Non-broadcast keyless Memcached bucket request found!")
		}
		srvIdx = routingInfo.MapKetama(req.Key)
	}

	if srvIdx == -1 {
		return routingInfo.deadQueue, nil
	}

	return routingInfo.queues[srvIdx], nil
}

// This immediately dispatches a request to the appropriate server based on the
//  currently available routing data.
func (c *Agent) dispatchDirect(req *memdQRequest) error {
	for {
		pipeline, err := c.routeRequest(req)
		if err != nil {
			return err
		}

		if pipeline == nil {
			// If no routing data exists this indicates that this Agent
			//   has been shut down!
			return ErrShutdown
		}

		if !pipeline.QueueRequest(req) {
			continue
		}

		break
	}
	return nil
}

// This function is meant to be used when a memdRequest is internally shuffled
//   around.  It currently simply calls dispatchDirect.
func (c *Agent) redispatchDirect(req *memdQRequest) {
	// Reschedule the operation
	err := c.dispatchDirect(req)
	if err != nil {
		panic("dispatchDirect errored during redispatch.")
	}
}
