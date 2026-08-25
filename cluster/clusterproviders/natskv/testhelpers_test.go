package natskv

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/require"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/cluster"
	"github.com/awevoke/protoactor-go/remote"
)

// startEmbeddedNATS starts an embedded NATS server with JetStream enabled.
func startEmbeddedNATS(t *testing.T) *server.Server {
	t.Helper()

	opts := &server.Options{
		JetStream: true,
		Port:      -1,
		StoreDir:  t.TempDir(),
	}

	srv, err := server.NewServer(opts)
	require.NoError(t, err, "failed to create NATS server")

	srv.Start()
	t.Cleanup(func() {
		srv.Shutdown()
		srv.WaitForShutdown()
	})

	if !srv.ReadyForConnections(5 * time.Second) {
		t.Fatal("NATS server not ready")
	}

	return srv
}

// connectNATS connects to the given embedded NATS server and returns a connection and JetStream handle.
func connectNATS(t *testing.T, srv *server.Server) (*nats.Conn, jetstream.JetStream) {
	t.Helper()

	nc, err := nats.Connect(srv.ClientURL())
	require.NoError(t, err, "failed to connect to NATS")
	t.Cleanup(func() { nc.Close() })

	js, err := jetstream.New(nc)
	require.NoError(t, err, "failed to create JetStream context")

	return nc, js
}

// setupClusterWithKindsEmbedded creates a provider, actor system, and cluster
// with registered kinds for testing using the embedded NATS server.
func setupClusterWithKindsEmbedded(t *testing.T, srv *server.Server, clusterName string, kinds []*cluster.Kind, opts ...Option) (*Provider, *cluster.Cluster) {
	t.Helper()

	nc, _ := connectNATS(t, srv)

	p, err := New(nc, opts...)
	require.NoError(t, err)

	system := actor.NewActorSystem()
	remoteConfig := remote.Configure("127.0.0.1", 0)
	clusterConfig := cluster.Configure(clusterName, p, p.IdentityLookup(), remoteConfig,
		cluster.WithKinds(kinds...),
	)
	c := cluster.NewCluster(system, clusterConfig)
	c.Remote = remote.NewRemote(system, remoteConfig)

	return p, c
}

// natskvTestEnv carries what a counting test needs alongside the lookup: the
// connection its KV traffic crosses and the bucket names the fixture created.
// It exists instead of accessors on IdentityLookup because these tests are
// `package natskv` -- production code gains nothing for a test's benefit.
type natskvTestEnv struct {
	conn           *nats.Conn
	js             jetstream.JetStream
	identityBucket string
	trackingBucket string
}

// identityLookupTestConfig is newIdentityLookupForTest's option target.
type identityLookupTestConfig struct {
	clusterName  string
	tombstoneTTL time.Duration
	// bucketMarkerTTL is the TTL the BUCKETS are created with, when it needs
	// to differ from the configured TombstoneTTL. nil means "same as
	// tombstoneTTL", which is what production does. Decoupling the two is the
	// only way to build the real fallback shape: a bucket whose stream carries
	// AllowMsgTTL: false while config.TombstoneTTL is positive.
	bucketMarkerTTL  *time.Duration
	markerTTLEnabled *bool // nil = whatever the server actually supports
}

// natskvTestOption configures newIdentityLookupForTest.
type natskvTestOption func(*identityLookupTestConfig)

// withMarkerTTLEnabled forces the "did this bucket really get marker TTLs"
// answer, so tombstoneOpts can be exercised for BOTH server shapes without
// needing a downlevel server: the fallback path's whole hazard is that a
// PurgeTTL on a bucket without AllowMsgTTL fails every delete.
func withMarkerTTLEnabled(enabled bool) natskvTestOption {
	return func(c *identityLookupTestConfig) { c.markerTTLEnabled = &enabled }
}

// withTombstoneTTL overrides the marker TTL, for the bounded-marker test.
func withTombstoneTTL(d time.Duration) natskvTestOption {
	return func(c *identityLookupTestConfig) { c.tombstoneTTL = d }
}

// withBucketMarkerTTL overrides the TTL the buckets are CREATED with, leaving
// config.TombstoneTTL alone. Passing 0 produces a bucket whose stream really
// has AllowMsgTTL: false while the configured TombstoneTTL stays positive --
// the exact shape a server without marker-TTL support leaves behind, and the
// only shape in which dropping markerTTLEnabled from tombstoneOpts' gate is
// observable.
func withBucketMarkerTTL(d time.Duration) natskvTestOption {
	return func(c *identityLookupTestConfig) { c.bucketMarkerTTL = &d }
}

// newIdentityLookupForTest builds an IdentityLookup against an embedded NATS
// server with both KV buckets provisioned through the production
// createBucketWithMarkerTTL, so the marker-TTL path under test is the real one.
//
// Modelled on the &IdentityLookup{...} literals the package's tests already use
// (buildBareIdentityLookup in reap_config_test.go and eleven inline siblings) --
// same fields plus the one this branch adds.
func newIdentityLookupForTest(t *testing.T, opts ...natskvTestOption) (*IdentityLookup, *natskvTestEnv) {
	t.Helper()

	tc := identityLookupTestConfig{
		clusterName:  "testcluster",
		tombstoneTTL: defaultTombstoneTTL,
	}

	for _, o := range opts {
		o(&tc)
	}

	srv := startEmbeddedNATS(t)
	nc, js := connectNATS(t, srv)
	ctx := context.Background()

	cfg := newDefaultConfig()
	cfg.TombstoneTTL = tc.tombstoneTTL

	bucketTTL := tc.tombstoneTTL
	if tc.bucketMarkerTTL != nil {
		bucketTTL = *tc.bucketMarkerTTL
	}

	// Unique per test so t.Parallel() cases cannot collide on one server.
	identityBucket := cfg.identityBucketName(tc.clusterName) + "_" + strings.ReplaceAll(t.Name(), "/", "_")
	trackingBucket := "protoactor_" + tc.clusterName + "_identities_tracking_" + strings.ReplaceAll(t.Name(), "/", "_")

	identities, ttlEnabled, err := createBucketWithMarkerTTL(ctx, js, jetstream.KeyValueConfig{
		Bucket:   identityBucket,
		Replicas: 1,
	}, bucketTTL, discardLogger())
	require.NoError(t, err)

	tracker, _, err := createBucketWithMarkerTTL(ctx, js, jetstream.KeyValueConfig{
		Bucket:   trackingBucket,
		Replicas: 1,
	}, bucketTTL, discardLogger())
	require.NoError(t, err)

	if tc.markerTTLEnabled != nil {
		ttlEnabled = *tc.markerTTLEnabled
	}

	il := &IdentityLookup{
		identities:       identities,
		memberTracker:    tracker,
		config:           cfg,
		semaphore:        make(chan struct{}, 200),
		memberID:         tc.clusterName + "_node1",
		markerTTLEnabled: ttlEnabled,
		// now is not optional: recordWriteFailure and janitorSweep both call
		// il.now(), so a zero-value clock is a nil-func panic, not a default.
		now: time.Now,
	}

	return il, &natskvTestEnv{
		conn:           nc,
		js:             js,
		identityBucket: identityBucket,
		trackingBucket: trackingBucket,
	}
}

// discardLogger is a *slog.Logger whose output goes nowhere; the fixture passes
// it into createBucketWithMarkerTTL, whose only output is a fallback warning.
func discardLogger() *slog.Logger { return slog.New(slog.DiscardHandler) }

// streamMsgCount returns the number of messages retained by the named stream.
// For a KV bucket (History 1 => MaxMsgsPerSubject 1) that is exactly
// "live keys + retained delete/purge markers", which is the quantity this
// task is about.
func streamMsgCount(t *testing.T, js jetstream.JetStream, stream string) int {
	t.Helper()

	s, err := js.Stream(context.Background(), stream)
	require.NoError(t, err)

	info, err := s.Info(context.Background())
	require.NoError(t, err)

	return int(info.State.Msgs)
}

// lastMsgHeaders returns the headers of the last message stored on subject.
// Used to inspect the marker a delete left behind (KV-Operation, Nats-Rollup,
// Nats-TTL).
func lastMsgHeaders(t *testing.T, js jetstream.JetStream, stream, subject string) nats.Header {
	t.Helper()

	s, err := js.Stream(context.Background(), stream)
	require.NoError(t, err)

	msg, err := s.GetLastMsgForSubject(context.Background(), subject)
	require.NoError(t, err)

	return msg.Header
}

// jsAPICounter records every JetStream API request issued over a connection,
// so a test can pin that a code path makes no per-item round trips. It is the
// N+1 guard for the janitor sweep: KV reads and writes ride the $KV.> subjects,
// while anything that reaches for stream administration (PurgeDeletes issues
// one $JS.API.STREAM.PURGE.* per delete marker) shows up here.
type jsAPICounter struct {
	mu       sync.Mutex
	subjects []string
}

// newJSAPICounter attaches a $JS.API.> observer to nc for the test's lifetime.
func newJSAPICounter(t *testing.T, nc *nats.Conn) *jsAPICounter {
	t.Helper()

	c := &jsAPICounter{}

	sub, err := nc.Subscribe("$JS.API.>", func(m *nats.Msg) {
		c.mu.Lock()
		c.subjects = append(c.subjects, m.Subject)
		c.mu.Unlock()
	})
	require.NoError(t, err)
	require.NoError(t, nc.Flush())

	t.Cleanup(func() { _ = sub.Unsubscribe() })

	return c
}

// countPrefix returns how many observed API requests start with prefix. The
// caller must have flushed the connection first so that every request the code
// under test issued has been echoed back to the observer.
func (c *jsAPICounter) countPrefix(t *testing.T, prefix string) int {
	t.Helper()

	c.mu.Lock()
	defer c.mu.Unlock()

	n := 0

	for _, s := range c.subjects {
		if strings.HasPrefix(s, prefix) {
			n++
		}
	}

	return n
}
