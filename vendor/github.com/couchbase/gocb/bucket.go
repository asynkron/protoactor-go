package gocb

import (
	"gopkg.in/couchbase/gocbcore.v2"
	"math/rand"
	"time"
)

// Bucket is an interface representing a single bucket within a cluster.
type Bucket struct {
	cluster   *Cluster
	name      string
	password  string
	client    *gocbcore.Agent
	mtEnabled bool

	transcoder      Transcoder
	opTimeout       time.Duration
	bulkOpTimeout   time.Duration
	duraTimeout     time.Duration
	duraPollTimeout time.Duration
	viewTimeout     time.Duration
	n1qlTimeout     time.Duration
	ftsTimeout      time.Duration

	internal *BucketInternal
}

func createBucket(cluster *Cluster, config *gocbcore.AgentConfig) (*Bucket, error) {
	cli, err := gocbcore.CreateAgent(config)
	if err != nil {
		return nil, err
	}

	bucket := &Bucket{
		cluster:    cluster,
		name:       config.BucketName,
		password:   config.Password,
		client:     cli,
		mtEnabled:  config.UseMutationTokens,
		transcoder: &DefaultTranscoder{},

		opTimeout:       2500 * time.Millisecond,
		bulkOpTimeout:   10000 * time.Millisecond,
		duraTimeout:     40000 * time.Millisecond,
		duraPollTimeout: 100 * time.Millisecond,
		viewTimeout:     75 * time.Second,
		n1qlTimeout:     75 * time.Second,
		ftsTimeout:      75 * time.Second,
	}
	bucket.internal = &BucketInternal{
		b: bucket,
	}
	return bucket, nil
}

// OperationTimeout returns the maximum amount of time to wait for an operation to succeed.
func (b *Bucket) OperationTimeout() time.Duration {
	return b.opTimeout
}

// SetOperationTimeout sets the maximum amount of time to wait for an operation to succeed.
func (b *Bucket) SetOperationTimeout(timeout time.Duration) {
	b.opTimeout = timeout
}

// BulkOperationTimeout returns the maximum amount of time to wait for a bulk op to succeed.
func (b *Bucket) BulkOperationTimeout() time.Duration {
	return b.bulkOpTimeout
}

// SetBulkOperationTimeout sets the maxium amount of time to wait for a bulk op to succeed.
func (b *Bucket) SetBulkOperationTimeout(timeout time.Duration) {
	b.bulkOpTimeout = timeout
}

// DurabilityTimeout returns the maximum amount of time to wait for durability to succeed.
func (b *Bucket) DurabilityTimeout() time.Duration {
	return b.duraTimeout
}

// SetDurabilityTimeout sets the maximum amount of time to wait for durability to succeed.
func (b *Bucket) SetDurabilityTimeout(timeout time.Duration) {
	b.duraTimeout = timeout
}

// SetDurabilityTimeout returns the amount of time waiting between durability polls.
func (b *Bucket) DurabilityPollTimeout() time.Duration {
	return b.duraPollTimeout
}

// SetDurabilityTimeout sets the amount of time waiting between durability polls.
func (b *Bucket) SetDurabilityPollTimeout(timeout time.Duration) {
	b.duraPollTimeout = timeout
}

// ViewTimeout returns the maximum amount of time to wait for a view query to complete.
func (b *Bucket) ViewTimeout() time.Duration {
	return b.viewTimeout
}

// SetViewTimeout sets the maximum amount of time to wait for a view query to complete.
func (b *Bucket) SetViewTimeout(timeout time.Duration) {
	b.viewTimeout = timeout
}

// N1qlTimeout returns the maximum amount of time to wait for a N1QL query to complete.
func (b *Bucket) N1qlTimeout() time.Duration {
	return b.n1qlTimeout
}

// SetN1qlTimeout sets the maximum amount of time to wait for a N1QL query to complete.
func (b *Bucket) SetN1qlTimeout(timeout time.Duration) {
	b.n1qlTimeout = timeout
}

// SetTranscoder specifies a Transcoder to use when translating documents from their
//  raw byte format to Go types and back.
func (b *Bucket) SetTranscoder(transcoder Transcoder) {
	b.transcoder = transcoder
}

// InvalidateQueryCache forces the internal cache of prepared queries to be cleared.
//  Queries to be cached are controlled by the Adhoc() method of N1qlQuery.
func (b *Bucket) InvalidateQueryCache() {
	b.cluster.InvalidateQueryCache()
}

// Cas represents the specific state of a document on the cluster.
type Cas gocbcore.Cas
type pendingOp gocbcore.PendingOp

func (b *Bucket) getViewEp() (string, error) {
	capiEps := b.client.CapiEps()
	if len(capiEps) == 0 {
		return "", &clientError{"No available view nodes."}
	}
	return capiEps[rand.Intn(len(capiEps))], nil
}

func (b *Bucket) getMgmtEp() (string, error) {
	mgmtEps := b.client.MgmtEps()
	if len(mgmtEps) == 0 {
		return "", &clientError{"No available management nodes."}
	}
	return mgmtEps[rand.Intn(len(mgmtEps))], nil
}

func (b *Bucket) getN1qlEp() (string, error) {
	n1qlEps := b.client.N1qlEps()
	if len(n1qlEps) == 0 {
		return "", &clientError{"No available N1QL nodes."}
	}
	return n1qlEps[rand.Intn(len(n1qlEps))], nil
}

func (b *Bucket) getFtsEp() (string, error) {
	ftsEps := b.client.FtsEps()
	if len(ftsEps) == 0 {
		return "", &clientError{"No available FTS nodes."}
	}
	return ftsEps[rand.Intn(len(ftsEps))], nil
}

// Close the instance’s underlying socket resources.  Note that operations pending on the connection may fail.
func (b *Bucket) Close() {
	b.cluster.closeBucket(b)
	b.client.Close()
}

// IoRouter returns the underlying gocb agent managing connections.
func (b *Bucket) IoRouter() *gocbcore.Agent {
	return b.client
}

// *INTERNAL*
// Internal methods, not safe to be consumed by third parties.
func (b *Bucket) Internal() *BucketInternal {
	return b.internal
}

// Manager returns a BucketManager for performing management operations on this bucket.
func (b *Bucket) Manager(username, password string) *BucketManager {
	userPass := userPassPair{username, password}
	if username == "" || password == "" {
		if b.cluster.auth != nil {
			userPass = b.cluster.auth.bucketMgmt(b.name)
		}
	}

	return &BucketManager{
		bucket:   b,
		username: userPass.Username,
		password: userPass.Password,
	}
}
