package gocb

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"gopkg.in/couchbase/gocbcore.v2"
	"io/ioutil"
	"net/http"
	"sync"
	"time"
)

// Cluster represents a connection to a specific Couchbase cluster.
type Cluster struct {
	spec                 connSpec
	auth                 Authenticator
	connectTimeout       time.Duration
	serverConnectTimeout time.Duration
	n1qlTimeout          time.Duration
	ftsTimeout           time.Duration
	nmvRetryDelay        time.Duration
	tlsConfig            *tls.Config

	clusterLock sync.RWMutex
	queryCache  map[string]*n1qlCache
	bucketList  []*Bucket
}

// Connect creates a new Cluster object for a specific cluster.
func Connect(connSpecStr string) (*Cluster, error) {
	spec, err := parseConnSpec(connSpecStr)
	if err != nil {
		return nil, err
	}
	if spec.Bucket != "" {
		return nil, errors.New("Connection string passed to Connect() must not have any bucket specified!")
	}

	csResolveDnsSrv(&spec)

	// Get bootstrap_on option to determine which, if any, of the bootstrap nodes should be cleared
	switch spec.Options.Get("bootstrap_on") {
	case "http":
		spec.MemcachedHosts = nil
		if len(spec.HttpHosts) == 0 {
			return nil, errors.New("bootstrap_on=http but no HTTP hosts in connection string")
		}
	case "cccp":
		spec.HttpHosts = nil
		if len(spec.MemcachedHosts) == 0 {
			return nil, errors.New("bootstrap_on=cccp but no CCCP/Memcached hosts in connection string")
		}
	case "both":
	case "":
		// Do nothing
		break
	default:
		return nil, errors.New("bootstrap_on={http,cccp,both}")
	}

	cluster := &Cluster{
		spec:                 spec,
		connectTimeout:       60000 * time.Millisecond,
		serverConnectTimeout: 7000 * time.Millisecond,
		n1qlTimeout:          75 * time.Second,
		ftsTimeout:           75 * time.Second,
		nmvRetryDelay:        100 * time.Millisecond,

		queryCache: make(map[string]*n1qlCache),
	}
	return cluster, nil
}

// ConnectTimeout returns the maximum time to wait when attempting to connect to a bucket.
func (c *Cluster) ConnectTimeout() time.Duration {
	return c.connectTimeout
}

// SetConnectTimeout sets the maximum time to wait when attempting to connect to a bucket.
func (c *Cluster) SetConnectTimeout(timeout time.Duration) {
	c.connectTimeout = timeout
}

// ServerConnectTimeout returns the maximum time to attempt to connect to a single node.
func (c *Cluster) ServerConnectTimeout() time.Duration {
	return c.serverConnectTimeout
}

// SetServerConnectTimeout sets the maximum time to attempt to connect to a single node.
func (c *Cluster) SetServerConnectTimeout(timeout time.Duration) {
	c.serverConnectTimeout = timeout
}

// N1qlTimeout returns the maximum time to wait for a cluster-level N1QL query to complete.
func (c *Cluster) N1qlTimeout() time.Duration {
	return c.n1qlTimeout
}

// SetN1qlTimeout sets the maximum time to wait for a cluster-level N1QL query to complete.
func (c *Cluster) SetN1qlTimeout(timeout time.Duration) {
	c.n1qlTimeout = timeout
}

// FtsTimeout returns the maximum time to wait for a cluster-level FTS query to complete.
func (c *Cluster) FtsTimeout() time.Duration {
	return c.ftsTimeout
}

// SetFtsTimeout sets the maximum time to wait for a cluster-level FTS query to complete.
func (c *Cluster) SetFtsTimeout(timeout time.Duration) {
	c.ftsTimeout = timeout
}

// NmvRetryDelay returns the time to wait between retrying an operation due to not my vbucket.
func (c *Cluster) NmvRetryDelay() time.Duration {
	return c.nmvRetryDelay
}

// SetNmvRetryDelay sets the time to wait between retrying an operation due to not my vbucket.
func (c *Cluster) SetNmvRetryDelay(delay time.Duration) {
	c.nmvRetryDelay = delay
}

// InvalidateQueryCache forces the internal cache of prepared queries to be cleared.
func (c *Cluster) InvalidateQueryCache() {
	c.clusterLock.Lock()
	c.queryCache = make(map[string]*n1qlCache)
	c.clusterLock.Unlock()
}

func specToHosts(spec connSpec) ([]string, []string, bool) {
	var memdHosts []string
	var httpHosts []string

	for _, specHost := range spec.HttpHosts {
		httpHosts = append(httpHosts, specHost.HostPort())
	}

	for _, specHost := range spec.MemcachedHosts {
		memdHosts = append(memdHosts, specHost.HostPort())
	}

	return memdHosts, httpHosts, spec.Scheme.IsSSL()
}

func (c *Cluster) makeAgentConfig(bucket, password string, mt bool) (*gocbcore.AgentConfig, error) {
	authFn := func(srv gocbcore.AuthClient, deadline time.Time) error {
		// Build PLAIN auth data
		userBuf := []byte(bucket)
		passBuf := []byte(password)
		authData := make([]byte, 1+len(userBuf)+1+len(passBuf))
		authData[0] = 0
		copy(authData[1:], userBuf)
		authData[1+len(userBuf)] = 0
		copy(authData[1+len(userBuf)+1:], passBuf)

		// Execute PLAIN authentication
		_, err := srv.ExecSaslAuth([]byte("PLAIN"), authData, deadline)

		return err
	}

	memdHosts, httpHosts, isSslHosts := specToHosts(c.spec)

	var tlsConfig *tls.Config
	if isSslHosts {

		certpath := c.spec.Options.Get("certpath")

		tlsConfig = &tls.Config{}
		if certpath == "" {
			tlsConfig.InsecureSkipVerify = true
		} else {
			cacert, err := ioutil.ReadFile(certpath)
			if err != nil {
				return nil, err
			}

			roots := x509.NewCertPool()
			ok := roots.AppendCertsFromPEM(cacert)
			if !ok {
				return nil, ErrInvalidCert
			}
			tlsConfig.RootCAs = roots
		}
	}

	return &gocbcore.AgentConfig{
		MemdAddrs:            memdHosts,
		HttpAddrs:            httpHosts,
		TlsConfig:            tlsConfig,
		BucketName:           bucket,
		Password:             password,
		AuthHandler:          authFn,
		UseMutationTokens:    mt,
		ConnectTimeout:       c.connectTimeout,
		ServerConnectTimeout: c.serverConnectTimeout,
		NmvRetryDelay:        c.nmvRetryDelay,
	}, nil
}

// Authenticate specifies an Authenticator interface to use to authenticate with cluster services.
func (c *Cluster) Authenticate(auth Authenticator) error {
	c.auth = auth
	return nil
}

func (c *Cluster) openBucket(bucket, password string, mt bool) (*Bucket, error) {
	if password == "" {
		if c.auth != nil {
			password = c.auth.bucketMemd(bucket)
		}
	}

	agentConfig, err := c.makeAgentConfig(bucket, password, mt)
	if err != nil {
		return nil, err
	}

	b, err := createBucket(c, agentConfig)
	if err != nil {
		return nil, err
	}

	c.clusterLock.Lock()
	c.bucketList = append(c.bucketList, b)
	c.clusterLock.Unlock()

	return b, nil
}

// OpenBucket opens a new connection to the specified bucket.
func (c *Cluster) OpenBucket(bucket, password string) (*Bucket, error) {
	return c.openBucket(bucket, password, false)
}

// OpenBucketWithMt opens a new connection to the specified bucket and enables mutation tokens.
// MutationTokens allow you to execute queries and durability requirements with very specific
// operation-level consistency.
func (c *Cluster) OpenBucketWithMt(bucket, password string) (*Bucket, error) {
	return c.openBucket(bucket, password, true)
}

func (c *Cluster) closeBucket(bucket *Bucket) {
	c.clusterLock.Lock()
	for i, e := range c.bucketList {
		if e == bucket {
			c.bucketList = append(c.bucketList[0:i], c.bucketList[i+1:]...)
			break
		}
	}
	c.clusterLock.Unlock()
}

// Manager returns a ClusterManager object for performing cluster management operations on this cluster.
func (c *Cluster) Manager(username, password string) *ClusterManager {
	userPass := userPassPair{username, password}
	if username == "" || password == "" {
		if c.auth != nil {
			userPass = c.auth.clusterMgmt()
		}
	}

	_, httpHosts, isSslHosts := specToHosts(c.spec)
	var mgmtHosts []string

	for _, host := range httpHosts {
		if isSslHosts {
			mgmtHosts = append(mgmtHosts, "https://"+host)
		} else {
			mgmtHosts = append(mgmtHosts, "http://"+host)
		}
	}

	var tlsConfig *tls.Config
	if isSslHosts {
		tlsConfig = c.tlsConfig
		if tlsConfig == nil {
			tlsConfig = &tls.Config{
				InsecureSkipVerify: true,
			}
		}
	}

	return &ClusterManager{
		hosts:    mgmtHosts,
		username: userPass.Username,
		password: userPass.Password,
		httpCli: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: tlsConfig,
			},
		},
	}
}

// StreamingBucket represents a bucket connection used for streaming data over DCP.
type StreamingBucket struct {
	client *gocbcore.Agent
}

// IoRouter returns the underlying gocb agent managing connections.
func (b *StreamingBucket) IoRouter() *gocbcore.Agent {
	return b.client
}

// OpenBucket opens a new connection to the specified bucket for the purpose of streaming data.
func (c *Cluster) OpenStreamingBucket(streamName, bucket, password string) (*StreamingBucket, error) {
	agentConfig, err := c.makeAgentConfig(bucket, password, false)
	if err != nil {
		return nil, err
	}
	cli, err := gocbcore.CreateDcpAgent(agentConfig, streamName)
	if err != nil {
		return nil, err
	}

	return &StreamingBucket{
		client: cli,
	}, nil
}

func (c *Cluster) randomBucket() (*Bucket, error) {
	c.clusterLock.RLock()
	if len(c.bucketList) == 0 {
		c.clusterLock.RUnlock()
		return nil, ErrNoOpenBuckets
	}
	bucket := c.bucketList[0]
	c.clusterLock.RUnlock()
	return bucket, nil
}
