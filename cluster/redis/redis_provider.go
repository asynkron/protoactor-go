package redis

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/otherview/protoactor-go/cluster"
	"github.com/otherview/protoactor-go/eventstream"
	"github.com/otherview/protoactor-go/log"
)

var (
	plog                 = log.New(log.DebugLevel, "[CLUSTER] [REDIS]")
	clusterTTLErrorMutex = new(sync.Mutex)
	shutdownMutex        = new(sync.Mutex)
	deregisteredMutex    = new(sync.Mutex)
)

type RedisProvider struct {
	deregistered          bool
	shutdown              bool
	monitoringStatus      bool
	id                    string
	clusterName           string
	address               string
	port                  int
	knownKinds            []string
	knownNodes            map[string]*NodeModel
	redisClient           *redis.Pool
	refreshTTL            time.Duration
	expiration            int // after this time, redis will evict the key - in seconds
	statusValue           cluster.MemberStatusValue
	statusValueSerializer cluster.MemberStatusValueSerializer
	clusterTTLError       error
	clusterMonitorError   error
}

// New creates a RedisProvider that connects locally
func New() *RedisProvider {
	return NewWithConfig(
		"tcp",
		"localhost:6378",
		redis.DialDatabase(0),
		redis.DialConnectTimeout(10*time.Second),
		redis.DialReadTimeout(10*time.Second),
		redis.DialWriteTimeout(10*time.Second),
	)
}

// NewWithConfig creates a RedisProvider that connects to a given server
func NewWithConfig(network string, address string, options ...redis.DialOption) *RedisProvider {
	pool := &redis.Pool{
		MaxIdle:     5,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial(network, address, options...)
			if err != nil {
				return nil, err
			}
			return c, err
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
	}

	return NewWithPool(pool, 2*time.Second)
}

// NewWithPool creates a RedisProvider that uses a specified Pool
func NewWithPool(pool *redis.Pool, refreshTTL time.Duration) *RedisProvider {
	p := &RedisProvider{
		redisClient:      pool,
		refreshTTL:       refreshTTL,
		monitoringStatus: false,
	}
	return p
}

func (p *RedisProvider) RegisterMember(clusterName string, address string, port int, knownKinds []string,
	statusValue cluster.MemberStatusValue, serializer cluster.MemberStatusValueSerializer) error {
	p.id = fmt.Sprintf("%v@%v:%v", clusterName, address, port)
	p.clusterName = clusterName
	p.address = address
	p.port = port
	p.knownKinds = knownKinds
	p.statusValue = statusValue
	p.statusValueSerializer = serializer
	p.deregistered = false
	p.shutdown = false
	p.expiration = 10

	p.UpdateTTL()
	return nil
}

// DeregisterMember set the shutdown to true preventing any more TTL updates
func (p *RedisProvider) DeregisterMember() error {
	deregisteredMutex.Lock()
	defer deregisteredMutex.Unlock()
	p.deregistered = true

	return nil
}

// Shutdown set the shutdown to true preventing any more TTL updates
func (p *RedisProvider) Shutdown() error {
	shutdownMutex.Lock()
	defer shutdownMutex.Unlock()
	p.shutdown = true

	return nil
}

// UpdateTTL periodically updates the corresponding key in redis
// this key has an expiry period, if the node stops updating it, it will disappear
func (p *RedisProvider) UpdateTTL() {
	go func() {
		for !p.isShutdown() && !p.isDeregistered() {
			// create current Node - the NodeID is the key in Redis
			node := NewNode(p.clusterName, p.address, p.port, p.knownKinds)

			marshaled, err := json.Marshal(node)
			if err != nil {
				plog.Error("Error marshelling node", log.Error(err),
					log.String("RedisNodeMemberKey", node.ID),
					log.Object("RedisNodeMemberKey", node))

				clusterTTLErrorMutex.Lock()
				p.clusterTTLError = err
				clusterTTLErrorMutex.Unlock()

				time.Sleep(p.refreshTTL)
				continue
			}
			nodeString := string(marshaled)

			clusterTTLErrorMutex.Lock()
			p.clusterTTLError = p.updateMemberStatus(node, nodeString)
			clusterTTLErrorMutex.Unlock()

			time.Sleep(p.refreshTTL)
		}
	}()
}

// updateMemberStatus updates the node with Expiry
func (p *RedisProvider) updateMemberStatus(node *NodeModel, nodeString string) error {
	redisConn := p.redisClient.Get()
	defer redisConn.Close()

	// update key with expiry
	_, err := redisConn.Do("SET", node.ID, nodeString, "EX", p.expiration)
	if err != nil {
		plog.Error("Error setting the key", log.Error(err),
			log.String("RedisNodeMemberKey", node.ID),
			log.Object("RedisNodeMemberKey", node))
		return err
	}
	return err
}

func (p *RedisProvider) UpdateMemberStatusValue(statusValue cluster.MemberStatusValue) error {
	p.statusValue = statusValue
	if p.statusValue == nil {
		return nil
	}
	return nil
}

// MonitorMemberStatusChanges creates a go routine that continuously checks for cluster updates
func (p *RedisProvider) MonitorMemberStatusChanges() {
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
func (p *RedisProvider) GetHealthStatus() error {
	var err error
	clusterTTLErrorMutex.Lock()
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

func (p *RedisProvider) isShutdown() bool {
	shutdownMutex.Lock()
	defer shutdownMutex.Unlock()
	return p.shutdown
}

func (p *RedisProvider) isDeregistered() bool {
	deregisteredMutex.Lock()
	defer deregisteredMutex.Unlock()
	return p.deregistered
}

// monitorStatuses checks for node changes in the cluster
func (p *RedisProvider) monitorStatuses() {

	redisNodes, err := p.getRegisteredNodes()
	if err != nil {
		plog.Error("Failure refreshing nodes from redis.", log.Error(err))
		p.clusterMonitorError = err
		time.Sleep(p.refreshTTL)
		return
	}

	p.knownNodes = redisNodes

	// we should probably check if the cluster needs to be updated..
	res := make(cluster.ClusterTopologyEvent, len(p.knownNodes))
	i := 0
	for _, node := range p.knownNodes {
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
		res[i] = ms
		i++
	}
	p.clusterMonitorError = nil
	// publish the current cluster topology onto the event stream
	eventstream.Publish(res)

}

// getRegisteredNodes returns the nodes from redis
func (p *RedisProvider) getRegisteredNodes() (map[string]*NodeModel, error) {
	redisConn := p.redisClient.Get()
	defer redisConn.Close()

	// search existing keys for the cluster base name
	response, err := redis.Values(redisConn.Do("SCAN", 0, "MATCH", fmt.Sprintf("%s*", p.clusterName)))
	if err != nil {
		return nil, err
	}

	redisNodes, err := redis.Strings(response[1], nil)
	if err != nil {
		return nil, err
	}

	registeredNodes := make(map[string]*NodeModel, len(redisNodes))

	// get the node data
	for _, nodeKey := range redisNodes {
		nodeRedisData, err := redis.Bytes(redisConn.Do("GET", nodeKey))
		if err != nil {
			return registeredNodes, err
		}

		node := &NodeModel{}
		err = json.Unmarshal(nodeRedisData, node)
		if err != nil {
			return registeredNodes, err
		}

		registeredNodes[node.ID] = node
	}

	return registeredNodes, err
}

func (p *RedisProvider) deregisterService() {
	p.deregistered = true
}
