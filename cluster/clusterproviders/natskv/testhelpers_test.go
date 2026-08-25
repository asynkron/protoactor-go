package natskv

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
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

	// provider and memberBucket are non-nil only with withMemberBucket. The
	// janitor's membership half needs both: a Provider to hang
	// MemberKeysSnapshot off, and a real members bucket for it to enumerate.
	provider         *Provider
	memberBucket     jetstream.KeyValue
	memberBucketName string
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
	// memberBucket asks for a real Provider carrying a real members bucket.
	// Off by default: most tests never reach the provider, and creating the
	// bucket costs a round trip.
	memberBucket bool
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

// withMemberBucket attaches a real Provider with a real members KV bucket to
// the lookup, so a test can drive the half of janitorSweep that asks which
// members still exist. Without it the fixture's provider field is nil, because
// newIdentityLookupForTest builds the IdentityLookup as a struct literal
// rather than through Setup.
func withMemberBucket() natskvTestOption {
	return func(c *identityLookupTestConfig) { c.memberBucket = true }
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

	env := &natskvTestEnv{
		conn:           nc,
		js:             js,
		identityBucket: identityBucket,
		trackingBucket: trackingBucket,
	}

	if tc.memberBucket {
		provider, err := New(nc)
		require.NoError(t, err)

		provider.clusterName = tc.clusterName
		provider.config = cfg

		memberBucketName := "protoactor_members_" + strings.ReplaceAll(t.Name(), "/", "_")

		memberBucket, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
			Bucket:   memberBucketName,
			Replicas: 1,
		})
		require.NoError(t, err)

		provider.memberBucket = memberBucket
		il.provider = provider
		env.provider = provider
		env.memberBucket = memberBucket
		env.memberBucketName = memberBucketName
	}

	return il, env
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

// syncObserver is a SYNCHRONOUS subscription plus a drained record of every
// message it has seen. The counters below are all built on it.
//
// Synchronous is the whole point. nc.Flush() is a PING/PONG round trip, so when
// it returns the server has processed -- and the client has READ -- everything
// this connection published before it. For an ASYNC subscription that is not
// enough: the callback runs on its own goroutine fed from the read loop, so a
// flushed publish can still be sitting in the client's buffer when the test
// reads the count, and the count comes back short. A sync subscription's
// pending queue is filled by the read loop itself, before the PONG is seen, so
// flush-then-drain is exact rather than probable.
type syncObserver struct {
	nc   *nats.Conn
	sub  *nats.Subscription
	msgs []*nats.Msg
}

// newSyncObserver attaches an observer for subject to nc for the test's
// lifetime.
func newSyncObserver(t *testing.T, nc *nats.Conn, subject string) *syncObserver {
	t.Helper()

	sub, err := nc.SubscribeSync(subject)
	require.NoError(t, err)
	require.NoError(t, nc.Flush())

	t.Cleanup(func() { _ = sub.Unsubscribe() })

	return &syncObserver{nc: nc, sub: sub}
}

// drain flushes the connection and then pulls every message the subscription
// has queued, so that everything published so far is recorded.
func (o *syncObserver) drain(t *testing.T) {
	t.Helper()

	require.NoError(t, o.nc.Flush())

	for {
		pending, _, err := o.sub.Pending()
		require.NoError(t, err)

		if pending == 0 {
			return
		}

		msg, err := o.sub.NextMsg(5 * time.Second)
		require.NoError(t, err)

		o.msgs = append(o.msgs, msg)
	}
}

// all drains and returns everything observed so far.
func (o *syncObserver) all(t *testing.T) []*nats.Msg {
	t.Helper()

	o.drain(t)

	return o.msgs
}

// reset drains and then discards, so a test can count only the messages one
// specific call produces. The drain matters: without it, traffic from the setup
// phase would land in the window being measured.
func (o *syncObserver) reset(t *testing.T) {
	t.Helper()

	o.drain(t)

	o.msgs = nil
}

// jsAPICounter records every JetStream API request issued over a connection,
// so a test can pin that a code path makes no per-item round trips. It is the
// N+1 guard for the janitor sweep: KV reads and writes ride the $KV.> subjects,
// while anything that reaches for stream administration (PurgeDeletes issues
// one $JS.API.STREAM.PURGE.* per delete marker) shows up here.
type jsAPICounter struct {
	obs *syncObserver
}

// newJSAPICounter attaches a $JS.API.> observer to nc for the test's lifetime.
func newJSAPICounter(t *testing.T, nc *nats.Conn) *jsAPICounter {
	t.Helper()

	return &jsAPICounter{obs: newSyncObserver(t, nc, "$JS.API.>")}
}

// countPrefix returns how many observed API requests start with prefix.
func (c *jsAPICounter) countPrefix(t *testing.T, prefix string) int {
	t.Helper()

	n := 0

	for _, m := range c.obs.all(t) {
		if strings.HasPrefix(m.Subject, prefix) {
			n++
		}
	}

	return n
}

// reset discards everything observed so far.
func (c *jsAPICounter) reset(t *testing.T) {
	t.Helper()

	c.obs.reset(t)
}

// subjectCounter is jsAPICounter's shape over an arbitrary subject, for the
// side of a KV operation that never crosses $JS.API: a KV Put is a JetStream
// PUBLISH to $KV.<bucket>.<key> (nats.go kvs.Put -> js.Publish) and a KV
// Delete/Purge is a PUBLISH of a header-only message to the same subject, so
// the WRITE count of a code path is counted here while its READ count (a
// DIRECT.GET) is counted on jsAPICounter.
//
// It also exposes each observed message's payload size, which is what makes
// "the tracking write no longer grows with the number of tracked grains"
// checkable rather than asserted.
type subjectCounter struct {
	obs *syncObserver
}

// newSubjectCounter attaches an observer for subject to nc for the test's
// lifetime.
func newSubjectCounter(t *testing.T, nc *nats.Conn, subject string) *subjectCounter {
	t.Helper()

	return &subjectCounter{obs: newSyncObserver(t, nc, subject)}
}

// count returns how many messages have been observed.
func (c *subjectCounter) count(t *testing.T) int {
	t.Helper()

	return len(c.obs.all(t))
}

// maxPayload returns the largest observed payload in bytes. Zero when nothing
// was observed, which is indistinguishable from "every observed message was
// header-only" -- pair it with count.
func (c *subjectCounter) maxPayload(t *testing.T) int {
	t.Helper()

	most := 0

	for _, m := range c.obs.all(t) {
		if n := len(m.Data); n > most {
			most = n
		}
	}

	return most
}

// reset discards everything observed so far.
func (c *subjectCounter) reset(t *testing.T) {
	t.Helper()

	c.obs.reset(t)
}

// consumerCreateRecorder captures the CONSUMER.CREATE requests a code path
// issues, with the filter each one asked the SERVER to apply. Counting the
// creates proves how many enumerations ran; reading the filter proves the
// enumeration was scoped server-side rather than walking the whole bucket and
// filtering in the client.
type consumerCreateRecorder struct {
	obs *syncObserver
}

// consumerCreateRequest is the subset of jetstream's create-consumer request
// body these tests assert on.
type consumerCreateRequest struct {
	Stream string `json:"stream_name"`
	Config struct {
		FilterSubject  string   `json:"filter_subject"`
		FilterSubjects []string `json:"filter_subjects"`
	} `json:"config"`
}

// filters returns every filter subject the request asked for, whether it used
// the singular or the plural field.
func (r consumerCreateRequest) filters() []string {
	if len(r.Config.FilterSubjects) > 0 {
		return r.Config.FilterSubjects
	}

	if r.Config.FilterSubject == "" {
		return nil
	}

	return []string{r.Config.FilterSubject}
}

// newConsumerCreateRecorder attaches a $JS.API.CONSUMER.CREATE.> observer to nc
// for the test's lifetime.
func newConsumerCreateRecorder(t *testing.T, nc *nats.Conn) *consumerCreateRecorder {
	t.Helper()

	return &consumerCreateRecorder{obs: newSyncObserver(t, nc, "$JS.API.CONSUMER.CREATE.>")}
}

// created returns the requests seen so far.
func (c *consumerCreateRecorder) created(t *testing.T) []consumerCreateRequest {
	t.Helper()

	msgs := c.obs.all(t)
	out := make([]consumerCreateRequest, 0, len(msgs))

	for _, m := range msgs {
		var req consumerCreateRequest
		if err := json.Unmarshal(m.Data, &req); err != nil {
			continue
		}

		out = append(out, req)
	}

	return out
}

// reset discards everything observed so far.
func (c *consumerCreateRecorder) reset(t *testing.T) {
	t.Helper()

	c.obs.reset(t)
}
