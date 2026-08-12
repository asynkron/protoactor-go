//go:build soak

// Package natskv soak_test.go is a chaos-soak harness for the natskv identity
// store's restart-resilience self-healing mechanisms. It is guarded by the
// `soak` build tag so it NEVER runs in normal CI: it runs for minutes-to-hours
// and deliberately churns cluster membership under continuous load.
//
// Run locally:
//
//	SOAK_DURATION=3m go test -tags soak -run TestSoak_RestartResilience \
//	    ./cluster/clusterproviders/natskv/ -v -timeout 0
//
// The real pre-merge run passes SOAK_DURATION=Nh.
//
// Environment:
//
//	SOAK_DURATION  test duration (default 10m). Any time.ParseDuration value.
//	SOAK_MEMBERS   steady-state member count (default 4, min 2).
//	SOAK_SEED      RNG seed for reproducibility (default: derived from time,
//	               PRINTED at start).
//
// Design: one embedded JetStream NATS server, N in-process full cluster
// members sharing the same NATS + buckets. A chaos loop randomly kills /
// starts / kill-restarts members (always keeping >=1 alive). Several load
// loops continuously Get a fixed set of singleton identities through random
// live members. Three safety invariants are asserted continuously and fail
// the test IMMEDIATELY with full context on violation:
//
//	I1 NO DUPLICATE SINGLETON  -- a test grain registers (identity -> nonce) in
//	   a process-global registry on activation and deregisters on stop. A second
//	   LIVE registration for the same identity is FATAL.
//	I2 NO PERMANENT WEDGE      -- a per-identity consecutive-nil-Get span that
//	   exceeds 3 minutes while >=1 member was continuously alive is FATAL.
//	I3 NO ETERNAL ORPHANS      -- a background checker lists the identities
//	   bucket every 30s; any lock-only record older than 180s is FATAL, and the
//	   total record count must stay bounded.
package natskv

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/cluster"
	"github.com/awevoke/protoactor-go/remote"
)

// --- soak parameters -------------------------------------------------------

const (
	// soakKindCount and soakIDCount define the singleton identity matrix that
	// the load loops hammer: soakKindCount kinds x soakIDCount ids.
	soakKindCount = 5
	soakIDCount   = 3

	// soakLoadLoops is the number of concurrent Get loops.
	soakLoadLoops = 6

	// Invariant thresholds. These match the branch's documented worst-case
	// bounded latency (~45s activation + 60s reap + 30s janitor) with margin.
	// They are the PASS/FAIL gate -- do NOT relax them to make a run pass.
	wedgeThreshold  = 3 * time.Minute   // I2: max tolerated nil span while alive
	orphanThreshold = 180 * time.Second // I3: max age of a lock-only record

	// Chaos interval bounds (jittered).
	chaosMinInterval = 2 * time.Second
	chaosMaxInterval = 20 * time.Second

	// Load-loop Get cadence bounds.
	loadMinInterval = 10 * time.Millisecond
	loadMaxInterval = 50 * time.Millisecond

	// I3 checker cadence.
	orphanCheckInterval = 30 * time.Second

	// I4: injected orphans must be cleaned within this window.
	// Budget: 60s hard reap + 30s janitor interval + 60s grace + 90s margin.
	orphanCleanDeadline = 240 * time.Second

	// orphanInjectionWeight: the chaos loop fires an orphan injection
	// action on roughly 1 in 4 chaos ticks (action 3 of 0..3).
	orphanInjectionWeight = 4

	// Prober cadence per injected key: how often to Get a shape-(a)/(b) key
	// so the reap-on-Get path is exercised in addition to the janitor.
	proberMinInterval = 10 * time.Second
	proberMaxInterval = 20 * time.Second
)

// soakOrphanKind is the kind name used exclusively by injected orphan records.
// It never appears in h.kinds, so the load loops never legitimately Get it,
// meaning shape (c) records can only be cleaned by the janitor two-sweep path.
const soakOrphanKind = "soakorphan"

// soakKinds returns the kind names used by the harness.
func soakKinds() []string {
	kinds := make([]string, soakKindCount)
	for i := range kinds {
		kinds[i] = fmt.Sprintf("SoakKind%d", i)
	}
	return kinds
}

// soakIdentities returns every (kind, id) pair the load loops target.
func soakIdentities() []*cluster.ClusterIdentity {
	var out []*cluster.ClusterIdentity
	for _, k := range soakKinds() {
		for i := 0; i < soakIDCount; i++ {
			out = append(out, cluster.NewClusterIdentity(fmt.Sprintf("id-%d", i), k))
		}
	}
	return out
}

// --- I1: process-global singleton registry ---------------------------------

// activationRegistry tracks the live activations observed by test grain actors.
// Each activation carries a unique nonce; a second LIVE registration for the
// same identity while the first is still registered is a duplicate-singleton
// violation (I1). It is a process-global (package-level) registry shared by all
// in-process members, which is exactly what makes cross-member double
// activation detectable.
type activationRegistry struct {
	mu   sync.Mutex
	live map[string]activationOwner // identityKey -> current owner
	// violation is set the instant a duplicate is detected. The main goroutine
	// polls it and fails the test with full context.
	violation atomic.Pointer[string]
}

type activationOwner struct {
	nonce   uint64
	address string
	pidID   string
	since   time.Time
}

var soakRegistry = &activationRegistry{live: make(map[string]activationOwner)}

// soakNonce hands out globally-unique activation nonces.
var soakNonce atomic.Uint64

func identityKeyOf(ci *cluster.ClusterIdentity) string {
	return ci.Kind + "/" + ci.Identity
}

// register records a new live activation. If an activation for the same
// identity is already live, it records a violation (I1) and returns false.
func (r *activationRegistry) register(key string, owner activationOwner) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.live[key]; ok {
		msg := fmt.Sprintf(
			"I1 DUPLICATE SINGLETON for %q: existing nonce=%d addr=%s pid=%s (live %s) "+
				"vs NEW nonce=%d addr=%s pid=%s -- two actors active simultaneously",
			key,
			existing.nonce, existing.address, existing.pidID, time.Since(existing.since).Round(time.Millisecond),
			owner.nonce, owner.address, owner.pidID,
		)
		r.violation.CompareAndSwap(nil, &msg)
		return false
	}
	r.live[key] = owner
	return true
}

// deregister removes a live activation, but only if the removing nonce still
// owns the slot (a slower Stopping for a superseded activation must not evict
// the successor).
func (r *activationRegistry) deregister(key string, nonce uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.live[key]; ok && existing.nonce == nonce {
		delete(r.live, key)
	}
}

func (r *activationRegistry) firstViolation() *string {
	return r.violation.Load()
}

// purgeAddress removes all live activations hosted at addr. Called when a member
// is stopped or hard-crashed: its actors are dead regardless of whether their
// Stopping handler ran (a hard crash shuts the ActorSystem down abruptly and
// never delivers Stopping). Without this, a legitimate reactivation on a
// surviving member would be misread as a duplicate of the crashed-but-still-
// registered actor.
//
// This does NOT weaken I1: a genuine duplicate is two grains on two LIVE members
// simultaneously. Neither of those addresses is purged while both members live,
// so the duplicate is still caught. purgeAddress only clears entries for an
// address whose member has already been torn down.
func (r *activationRegistry) purgeAddress(addr string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for key, owner := range r.live {
		if owner.address == addr {
			delete(r.live, key)
		}
	}
}

// soakGrainProps builds the test grain kind Props. On actor.Started it registers
// (identity -> nonce); on actor.Stopping it deregisters. The identity is derived
// from the actor's PID id, which is "Kind/Identity".
func soakGrainProps() *actor.Props {
	return actor.PropsFromFunc(func(ctx actor.Context) {
		switch ctx.Message().(type) {
		case *actor.Started:
			self := ctx.Self()
			key := self.Id // placement spawns grains with id == "Kind/Identity"
			owner := activationOwner{
				nonce:   soakNonce.Add(1),
				address: self.Address,
				pidID:   self.Id,
				since:   time.Now(),
			}
			// Stash the nonce on the actor via a closure-captured var is not
			// possible with PropsFromFunc (stateless), so re-derive on stop by
			// matching address+pidID is unreliable. Instead we store the nonce
			// in a per-PID side map keyed by the full PID string.
			soakOwnerByPID.Store(self.String(), owner)
			soakRegistry.register(key, owner)
		case *actor.Stopping:
			self := ctx.Self()
			key := self.Id
			if v, ok := soakOwnerByPID.LoadAndDelete(self.String()); ok {
				owner := v.(activationOwner)
				soakRegistry.deregister(key, owner.nonce)
			}
		}
	})
}

// soakOwnerByPID maps a full PID string to its activationOwner so Stopping can
// deregister the exact nonce it registered.
var soakOwnerByPID sync.Map

// --- orphan injection registry ---------------------------------------------

// orphanShape enumerates the three orphan record shapes.
type orphanShape int

const (
	// shapeDeadOwnerLock: lock-only with dead owner's memberID.
	// Cleaned by reap-on-Get (absent-owner branch, 30s) or janitor hard-reap (60s).
	shapeDeadOwnerLock orphanShape = iota
	// shapeLegacyLock: lock-only with no memberID (legacy format).
	// Cleaned by janitor hard-reap only (60s).
	shapeLegacyLock
	// shapeDeadActivation: completed activation pointing at a previously-killed
	// member's address/PID with a fake identity (soakorphan kind). Only cleaned
	// by the janitor two-sweep path because the load loops never Get soakorphan
	// identities.
	shapeDeadActivation
)

func (s orphanShape) String() string {
	switch s {
	case shapeDeadOwnerLock:
		return "dead-owner-lock"
	case shapeLegacyLock:
		return "legacy-lock"
	case shapeDeadActivation:
		return "dead-activation"
	default:
		return "unknown"
	}
}

// injectedOrphan describes a single injected orphan record.
type injectedOrphan struct {
	key       string      // NATS KV key written
	shape     orphanShape
	injectedT time.Time
	cleanedT  time.Time // zero if not yet cleaned
}

// orphanRegistry tracks all injected orphan records and their cleanup status.
// Concurrent-safe: the injector goroutine writes, the I4 checker goroutine reads.
type orphanRegistry struct {
	mu      sync.Mutex
	entries []*injectedOrphan

	// Counters by shape (monotonic, never decremented).
	injectedByShape [3]int64
	cleanedByShape  [3]int64

	// maxTTClean is the maximum observed time-to-clean across all cleaned orphans.
	maxTTClean time.Duration
}

func (r *orphanRegistry) record(o *injectedOrphan) {
	r.mu.Lock()
	r.entries = append(r.entries, o)
	r.injectedByShape[o.shape]++
	r.mu.Unlock()
}

// markCleaned records that orphan key was cleaned at t and returns the updated
// maxTTClean. No-op if already marked cleaned.
func (r *orphanRegistry) markCleaned(key string, t time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, o := range r.entries {
		if o.key == key && o.cleanedT.IsZero() {
			o.cleanedT = t
			r.cleanedByShape[o.shape]++
			ttc := t.Sub(o.injectedT)
			if ttc > r.maxTTClean {
				r.maxTTClean = ttc
			}
			return
		}
	}
}

// snapshot returns a copy of all entries (does not acquire a lock on callee).
func (r *orphanRegistry) snapshot() []*injectedOrphan {
	r.mu.Lock()
	out := make([]*injectedOrphan, len(r.entries))
	copy(out, r.entries)
	r.mu.Unlock()
	return out
}

func (r *orphanRegistry) totals() (injected [3]int64, cleaned [3]int64, maxTTC time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.injectedByShape, r.cleanedByShape, r.maxTTClean
}

// global orphan registry for the run (package-level so checkers can reach it).
var soakOrphanReg = &orphanRegistry{}

// --- reap/janitor log scraper ----------------------------------------------

// countingHandler is an slog.Handler that counts records whose message contains
// certain substrings, so the end-of-run report can show reap/janitor activity
// without a metrics backend. It forwards nothing (discard) to keep output quiet.
//
// Matched log lines (from natskv_identity.go and janitor.go):
//   - reap:    "natskv identity: reaping stale lock (hard age threshold)"
//              "natskv identity: reaping stale lock (owner absent)"
//              "natskv janitor: reaping aged lock"
//              "natskv janitor: reaping activation with absent member"
//   - janitor: "natskv janitor: reaping aged lock"
//              "natskv janitor: reaping activation with absent member"
//   - clean:   "natskv identity: cleaning stale activation after grace elapsed"
type countingHandler struct {
	reap    atomic.Int64 // any message containing "reaping"
	janitor atomic.Int64 // any message containing "natskv janitor:"
	clean   atomic.Int64 // any message containing "cleaning stale activation"
}

func (h *countingHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (h *countingHandler) Handle(_ context.Context, r slog.Record) error {
	msg := r.Message
	// Count janitor lines before reap so the janitor counter is a strict subset
	// of reap for the two reaping paths; however reap also catches Get-path reaps.
	if containsSub(msg, "natskv janitor:") {
		h.janitor.Add(1)
	}
	if containsSub(msg, "reaping") {
		h.reap.Add(1)
	}
	if containsSub(msg, "cleaning stale activation") {
		h.clean.Add(1)
	}
	return nil
}

func (h *countingHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *countingHandler) WithGroup(_ string) slog.Handler      { return h }

func containsSub(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// --- member management ------------------------------------------------------

// soakMember is one in-process cluster node.
type soakMember struct {
	id       int
	provider *Provider
	cluster  *cluster.Cluster
	address  string // ActorSystem address, e.g. 127.0.0.1:42107
	startedT time.Time
}

// killedMemberInfo records the identifiers of a member that has been stopped,
// so the orphan injector can reference its address/memberID without it being
// a live member.
type killedMemberInfo struct {
	memberID string
	address  string
}

// soakHarness owns the embedded NATS server, the shared config, and the set of
// live members. It provides start/stop primitives the chaos loop drives.
type soakHarness struct {
	t         *testing.T
	natsURL   string
	handler   *countingHandler
	opts      []Option
	kinds     []*cluster.Kind
	clusterNm string

	mu      sync.Mutex
	members map[int]*soakMember
	nextID  int

	// killedMembers is a FIFO log of recently stopped members (capped at 10).
	// Written under mu. The orphan injector reads under mu to pick a dead owner.
	killedMembers []killedMemberInfo

	// aliveSince tracks the wall-clock instant at which the cluster last became
	// non-empty (>=1 member). Reset to zero whenever the cluster is empty. Used
	// by I2 to only count nil spans while >=1 member was continuously alive.
	aliveSince atomic.Int64 // unix nanos, 0 == currently empty

	// Cumulative lifecycle counters for the resource telemetry line. started
	// and stopped are monotonic totals across the whole run; their difference
	// is the live member count. A goroutine count that climbs with stopped is
	// the fingerprint of a leaked per-member teardown.
	started atomic.Int64
	stopped atomic.Int64
}

func newSoakHarness(t *testing.T, natsURL string, handler *countingHandler, clusterNm string) *soakHarness {
	// Chaos-tuned timings: short-ish TTLs so departures are detected within the
	// test, but not so short that healthy refreshes flap. These sit inside the
	// branch's bounded-latency budget.
	opts := []Option{
		WithMemberTTL(6 * time.Second),
		WithRefreshInterval(2 * time.Second),
		WithLeaderTTL(10 * time.Second),
	}
	kinds := make([]*cluster.Kind, 0, soakKindCount)
	for _, k := range soakKinds() {
		kinds = append(kinds, cluster.NewKind(k, soakGrainProps()))
	}
	return &soakHarness{
		t:         t,
		natsURL:   natsURL,
		handler:   handler,
		opts:      opts,
		kinds:     kinds,
		clusterNm: clusterNm,
		members:   make(map[int]*soakMember),
	}
}

// remoteConfigOpts returns the remote configuration used by every soak member.
//
// gRPC client keepalive is enabled so that a ClientConn to a peer that has been
// hard-crashed is actively probed and torn down by gRPC instead of lingering in
// a permanent reconnect loop. This mirrors what a production deployment should
// configure and bounds the per-connection goroutine/socket footprint under
// sustained restart churn. It is a partial mitigation only: see the harness
// report for the residual endpointWriter ClientConn that is not reclaimed when a
// writer is terminated while still mid-connect (a remote-package concern).
func remoteConfigOpts() []remote.ConfigOption {
	return []remote.ConfigOption{
		remote.WithDialOptions(
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithKeepaliveParams(keepalive.ClientParameters{
				Time:                5 * time.Second,
				Timeout:             3 * time.Second,
				PermitWithoutStream: true,
			}),
		),
		// Bound the endpointWriter connect-retry window. A writer toward a
		// hard-crashed peer blocks its mailbox goroutine in initialize()'s
		// synchronous retry loop; a short window lets it exit and be terminated
		// (and its ClientConn closed) promptly instead of accumulating under
		// churn. These are transport-hygiene settings, not invariant thresholds.
		remote.WithMaxRetryCount(2),
		remote.WithRetryBaseDelay(200 * time.Millisecond),
		remote.WithRetryMaxDelay(1 * time.Second),
	}
}

// startMember boots one full cluster node joined to the shared NATS + buckets.
func (h *soakHarness) startMember() (*soakMember, error) {
	nc, err := nats.Connect(h.natsURL)
	if err != nil {
		return nil, fmt.Errorf("nats connect: %w", err)
	}
	p, err := New(nc, h.opts...)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("provider new: %w", err)
	}

	system := actor.NewActorSystem(actor.WithLoggerFactory(func(s *actor.ActorSystem) *slog.Logger {
		return slog.New(h.handler)
	}))
	remoteCfg := remote.Configure("127.0.0.1", 0, remoteConfigOpts()...)
	clusterCfg := cluster.Configure(h.clusterNm, p, p.IdentityLookup(), remoteCfg,
		cluster.WithKinds(h.kinds...))
	c := cluster.NewCluster(system, clusterCfg)

	if err := c.StartMember(); err != nil {
		nc.Close()
		return nil, fmt.Errorf("start member: %w", err)
	}

	h.mu.Lock()
	id := h.nextID
	h.nextID++
	m := &soakMember{id: id, provider: p, cluster: c, address: c.ActorSystem.Address(), startedT: time.Now()}
	h.members[id] = m
	if h.aliveSince.Load() == 0 {
		h.aliveSince.Store(time.Now().UnixNano())
	}
	h.mu.Unlock()
	h.started.Add(1)
	return m, nil
}

// stopMember shuts down a member and removes it from the live set. The graceful
// path exercises the full production teardown; the hard-crash path models a real
// process kill (Stopping is never delivered, so stale KV claims persist).
//
// Both paths then call reclaimInProcess. This is required because the harness
// runs every "member" as an in-process actor system sharing one OS process. A
// real crashed pod has its goroutines, gRPC client connections, and sockets
// reclaimed by the kernel the instant the process dies; an in-process crash
// simulation does not get that for free. Without explicit reclamation the run
// leaks ~30 goroutines and a gRPC ClientConn per crash (a wedged endpointWriter
// per dead peer address plus the member's own NATS connection), which is what
// OOM-killed the 4h EKS soak. Reclaiming here restores the OS-kill semantics the
// harness is meant to model, without weakening the crash's KV-side behavior.
func (h *soakHarness) stopMember(m *soakMember, graceful bool) {
	h.mu.Lock()
	delete(h.members, m.id)
	empty := len(h.members) == 0
	if empty {
		h.aliveSince.Store(0)
	}
	// Record the killed member for orphan injection. Cap at 10 entries so the
	// ring does not grow unboundedly on multi-hour runs.
	killed := killedMemberInfo{
		memberID: m.provider.identity.memberID,
		address:  m.address,
	}
	h.killedMembers = append(h.killedMembers, killed)
	if len(h.killedMembers) > 10 {
		h.killedMembers = h.killedMembers[len(h.killedMembers)-10:]
	}
	h.mu.Unlock()

	if graceful {
		m.cluster.Shutdown(true)
	} else {
		// Hard crash: tear down remote + actor system, kill provider without
		// IdentityLookup.Shutdown() so stale claims persist (models a real
		// process kill). Mirrors crashCluster in the reactivation integration test.
		m.cluster.Remote.Shutdown(false)
		m.cluster.ActorSystem.Shutdown()
		m.provider.shutdown.Store(true)
		if m.provider.cancel != nil {
			m.provider.cancel()
		}
		m.provider.wg.Wait()
	}

	h.reclaimInProcess(m, graceful)

	// The member is gone: its grains are dead. Purge any registry entries for
	// its address. On graceful shutdown the Stopping handlers already
	// deregistered them (this is then a no-op); on hard crash Stopping never
	// ran, so this is the only cleanup. Done AFTER teardown so no further
	// Started message can re-add an entry for this address.
	soakRegistry.purgeAddress(m.address)
	h.stopped.Add(1)
}

// reclaimInProcess releases the in-process resources that a real OS process
// death would reclaim but that an in-process crash simulation leaves dangling:
//
//   - The member's raw NATS connection. The provider treats the connection as
//     caller-owned (it never calls nc.Close), so the harness -- which opened it
//     in startMember -- must close it. This is required on BOTH the graceful and
//     crash paths; neither cluster.Shutdown nor the crash teardown closes it.
//   - The identity-lookup background janitor goroutine and the member-side NATS
//     subscriptions, which the graceful path stops via IdentityLookup.Shutdown
//     but the crash path deliberately skips.
//   - Every remaining local actor process. ActorSystem.Shutdown only closes the
//     stopper channel; it never delivers Stopped, so endpointWriter actors never
//     run closeClientConn and their gRPC ClientConn (plus its background
//     stream.Recv goroutine) leaks. Stopping each local process delivers the
//     Stopped these handlers need. Redundant on the graceful path (already
//     stopped) and harmless there.
func (h *soakHarness) reclaimInProcess(m *soakMember, graceful bool) {
	if !graceful {
		il := m.provider.identity
		if il != nil {
			if il.janitorStop != nil {
				il.janitorStopOnce.Do(func() { close(il.janitorStop) })
			}
			if il.activationSub != nil {
				_ = il.activationSub.Unsubscribe()
			}
			if il.poisonSub != nil {
				_ = il.poisonSub.Unsubscribe()
			}
		}
		// Stop any actors the bare ActorSystem.Shutdown left running so their
		// Stopped-driven cleanup (notably endpointWriter.closeClientConn) fires.
		reg := m.cluster.ActorSystem.ProcessRegistry
		for i := range reg.LocalPIDs.LocalPIDs {
			for item := range reg.LocalPIDs.LocalPIDs[i].IterBuffered() {
				if proc, ok := item.Val.(actor.Process); ok {
					proc.Stop(m.cluster.ActorSystem.NewLocalPID(item.Key))
				}
			}
		}
	}

	// Always close the harness-owned NATS connection; the provider never does.
	if m.provider.nc != nil {
		m.provider.nc.Close()
	}
}

// liveMembers returns a snapshot of currently-live members.
func (h *soakHarness) liveMembers() []*soakMember {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]*soakMember, 0, len(h.members))
	for _, m := range h.members {
		out = append(out, m)
	}
	return out
}

func (h *soakHarness) memberCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.members)
}

// aliveContinuouslyFor reports whether at least one member has been alive for
// the entire window ending now, i.e. the current alive-span is >= d.
func (h *soakHarness) aliveContinuouslyFor(d time.Duration) bool {
	since := h.aliveSince.Load()
	if since == 0 {
		return false
	}
	return time.Since(time.Unix(0, since)) >= d
}

// anyLiveIdentityLookup returns an IdentityLookup from a live member for I3 bucket
// inspection, or nil if the cluster is momentarily empty.
func (h *soakHarness) anyLiveIdentityLookup() *IdentityLookup {
	ms := h.liveMembers()
	if len(ms) == 0 {
		return nil
	}
	return ms[0].provider.IdentityLookup()
}

// pickDeadMember returns a recently-killed member's info, or zero if none yet.
func (h *soakHarness) pickDeadMember(rng *rand.Rand) (killedMemberInfo, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.killedMembers) == 0 {
		return killedMemberInfo{}, false
	}
	return h.killedMembers[rng.Intn(len(h.killedMembers))], true
}

// injectOrphan writes a synthetic orphan record directly into the identities
// bucket via a live member's IdentityLookup. The injected key uses soakOrphanKind
// so it never collides with the load-loop singletons. Returns the injectedOrphan
// descriptor, or nil if injection was skipped (no live member / no dead member).
//
// The three shapes match the real incident classes:
//   - shapeDeadOwnerLock:  {"lid":"<uuid>","mid":"<dead-memberID>"}
//   - shapeLegacyLock:     {"lid":"<uuid>"}
//   - shapeDeadActivation: {"pid":"<fake>","adr":"<fake>","mid":"<dead-memberID>"}
func (h *soakHarness) injectOrphan(rng *rand.Rand, shape orphanShape, seq int) *injectedOrphan {
	il := h.anyLiveIdentityLookup()
	if il == nil {
		return nil
	}
	dead, ok := h.pickDeadMember(rng)
	if !ok {
		return nil
	}

	// Build a unique identity key. The identity string encodes the shape and a
	// sequence number so concurrent injections never collide.
	identity := fmt.Sprintf("orphan-%s-%d", shape, seq)
	ci := cluster.NewClusterIdentity(identity, soakOrphanKind)
	key := kvKey(ci)

	var rec activationRecord
	switch shape {
	case shapeDeadOwnerLock:
		rec = activationRecord{
			LockID:   fmt.Sprintf("fake-lock-%d", seq),
			MemberID: dead.memberID,
		}
	case shapeLegacyLock:
		rec = activationRecord{
			LockID: fmt.Sprintf("fake-lock-legacy-%d", seq),
			// No MemberID: legacy format.
		}
	case shapeDeadActivation:
		rec = activationRecord{
			PidID:      fmt.Sprintf("fake-pid-%d", seq),
			PidAddress: dead.address,
			MemberID:   dead.memberID,
		}
	}

	data, err := json.Marshal(&rec)
	if err != nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use Create so we don't overwrite a concurrent injection with the same key.
	// If the key already exists (extremely unlikely given the seq counter), skip.
	_, err = il.identities.Create(ctx, key, data)
	if err != nil {
		return nil
	}

	o := &injectedOrphan{
		key:       key,
		shape:     shape,
		injectedT: time.Now(),
	}
	soakOrphanReg.record(o)
	return o
}

// --- load-loop metrics ------------------------------------------------------

type latencyStats struct {
	mu      sync.Mutex
	samples []time.Duration // all Get latencies (ns); bounded by trimming
	total   atomic.Int64
	nils    atomic.Int64
}

func (l *latencyStats) record(d time.Duration, gotNil bool) {
	l.total.Add(1)
	if gotNil {
		l.nils.Add(1)
	}
	l.mu.Lock()
	l.samples = append(l.samples, d)
	// Keep memory bounded on multi-hour runs: once we exceed 200k samples,
	// down-sample by keeping every other one. Percentiles remain representative.
	if len(l.samples) > 200000 {
		trimmed := l.samples[:0]
		for i := 0; i < len(l.samples); i += 2 {
			trimmed = append(trimmed, l.samples[i])
		}
		l.samples = trimmed
	}
	l.mu.Unlock()
}

func (l *latencyStats) percentiles() (p50, p99, max time.Duration) {
	l.mu.Lock()
	cp := make([]time.Duration, len(l.samples))
	copy(cp, l.samples)
	l.mu.Unlock()
	if len(cp) == 0 {
		return 0, 0, 0
	}
	sort.Slice(cp, func(i, j int) bool { return cp[i] < cp[j] })
	p50 = cp[len(cp)*50/100]
	p99 = cp[min(len(cp)*99/100, len(cp)-1)]
	max = cp[len(cp)-1]
	return
}

// --- I2: per-identity nil-span tracking ------------------------------------

// nilSpanTracker records, per identity, the wall-clock instant of the last
// successful (non-nil) Get. A consecutive-nil span is now - lastOK. I2 fails if
// a span exceeds wedgeThreshold while the cluster was continuously alive for
// that whole span.
type nilSpanTracker struct {
	mu     sync.Mutex
	lastOK map[string]time.Time // identityKey -> last non-nil Get time
}

func newNilSpanTracker() *nilSpanTracker {
	return &nilSpanTracker{lastOK: make(map[string]time.Time)}
}

func (n *nilSpanTracker) markOK(key string) {
	now := time.Now()
	n.mu.Lock()
	n.lastOK[key] = now
	n.mu.Unlock()
}

// ensureSeen initializes lastOK for a key at t if absent, so the first span is
// measured from when the harness started observing the key rather than the
// epoch.
func (n *nilSpanTracker) ensureSeen(key string, t time.Time) {
	n.mu.Lock()
	if _, ok := n.lastOK[key]; !ok {
		n.lastOK[key] = t
	}
	n.mu.Unlock()
}

// worstSpan returns the identity with the longest current nil span and that span.
func (n *nilSpanTracker) worstSpan() (string, time.Duration) {
	now := time.Now()
	n.mu.Lock()
	defer n.mu.Unlock()
	var worstKey string
	var worst time.Duration
	for k, t := range n.lastOK {
		if s := now.Sub(t); s > worst {
			worst = s
			worstKey = k
		}
	}
	return worstKey, worst
}

// --- the test ---------------------------------------------------------------

func TestSoak_RestartResilience(t *testing.T) {
	duration := envDuration("SOAK_DURATION", 10*time.Minute)
	members := envInt("SOAK_MEMBERS", 4)
	if members < 2 {
		members = 2
	}
	seed := envSeed("SOAK_SEED")

	t.Logf("=== natskv chaos soak ===")
	t.Logf("SOAK_SEED=%d (pass this to reproduce)", seed)
	t.Logf("SOAK_DURATION=%s SOAK_MEMBERS=%d", duration, members)
	t.Logf("invariants: I1 no-duplicate-singleton, I2 no-wedge (>%s), I3 no-orphans (>%s)",
		wedgeThreshold, orphanThreshold)

	// One embedded JetStream NATS server for the whole run.
	srv := startEmbeddedNATS(t)
	natsURL := srv.ClientURL()

	handler := &countingHandler{}
	clusterNm := "soak-" + strconv.FormatInt(seed, 36)
	h := newSoakHarness(t, natsURL, handler, clusterNm)

	// Reset package-level registries for this run.
	soakOrphanReg = &orphanRegistry{}
	soakRegistry = &activationRegistry{live: make(map[string]activationOwner)}

	// Fatal channel: any invariant violation sends full context here and the
	// main goroutine fails immediately.
	fatalCh := make(chan string, 8)

	// RNG is seed-deterministic and owned by the chaos goroutine only.
	rng := rand.New(rand.NewSource(seed))

	// Boot the steady-state membership.
	for i := 0; i < members; i++ {
		m, err := h.startMember()
		require.NoError(t, err, "initial member %d failed to start", i)
		t.Logf("[boot] started member m%d", m.id)
		time.Sleep(300 * time.Millisecond) // stagger for predictable leader election
	}

	// Warm up: let topology converge and prime activations before chaos.
	time.Sleep(3 * time.Second)

	deadline := time.Now().Add(duration)
	stop := make(chan struct{})
	var wg sync.WaitGroup

	stats := &latencyStats{}
	spans := newNilSpanTracker()
	ids := soakIdentities()
	startedAt := time.Now()
	logResourceTelemetry(t, h, startedAt) // baseline after warm-up, before chaos
	for _, ci := range ids {
		spans.ensureSeen(identityKeyOf(ci), startedAt)
	}

	// --- load loops ---
	for loop := 0; loop < soakLoadLoops; loop++ {
		wg.Add(1)
		go func(loopID int) {
			defer wg.Done()
			lrng := rand.New(rand.NewSource(seed + int64(loopID) + 1))
			for {
				select {
				case <-stop:
					return
				default:
				}
				live := h.liveMembers()
				if len(live) == 0 {
					time.Sleep(20 * time.Millisecond)
					continue
				}
				m := live[lrng.Intn(len(live))]
				ci := ids[lrng.Intn(len(ids))]
				key := identityKeyOf(ci)

				t0 := time.Now()
				pid := safeGet(m.cluster, ci)
				lat := time.Since(t0)
				stats.record(lat, pid == nil)

				if pid != nil {
					spans.markOK(key)
				}

				sleep := loadMinInterval + time.Duration(lrng.Int63n(int64(loadMaxInterval-loadMinInterval)))
				time.Sleep(sleep)
			}
		}(loop)
	}

	// orphanSeq is a monotonically-increasing counter for unique orphan keys.
	// Owned by the chaos goroutine; no concurrent access.
	var orphanSeq int

	// --- chaos loop ---
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			jitter := chaosMinInterval + time.Duration(rng.Int63n(int64(chaosMaxInterval-chaosMinInterval)))
			select {
			case <-stop:
				return
			case <-time.After(jitter):
			}

			live := h.liveMembers()
			// action 0-2: member lifecycle. action 3: orphan injection (~1-in-4).
			action := rng.Intn(orphanInjectionWeight)
			switch action {
			case 0: // kill a random member, keeping >=1 alive
				if len(live) <= 1 {
					// too few to kill; start one instead
					if m, err := h.startMember(); err == nil {
						t.Logf("[chaos %s] start m%d (count=%d) [kill skipped: only %d alive]",
							ts(), m.id, h.memberCount(), len(live))
					}
					continue
				}
				victim := live[rng.Intn(len(live))]
				graceful := rng.Intn(2) == 0
				h.stopMember(victim, graceful)
				t.Logf("[chaos %s] kill m%d graceful=%v (count=%d)", ts(), victim.id, graceful, h.memberCount())
			case 1: // start a replacement member (capped at 2*SOAK_MEMBERS)
				if len(live) >= 2*members {
					t.Logf("[chaos %s] start skipped: at cap (%d >= 2*%d)", ts(), len(live), members)
					continue
				}
				m, err := h.startMember()
				if err != nil {
					t.Logf("[chaos %s] start FAILED: %v", ts(), err)
					continue
				}
				t.Logf("[chaos %s] start m%d (count=%d)", ts(), m.id, h.memberCount())
			case 2: // kill-and-immediately-restart (cold-start race)
				if len(live) <= 1 {
					if m, err := h.startMember(); err == nil {
						t.Logf("[chaos %s] start m%d (count=%d) [restart skipped: only %d alive]",
							ts(), m.id, h.memberCount(), len(live))
					}
					continue
				}
				victim := live[rng.Intn(len(live))]
				h.stopMember(victim, false) // hard crash
				m, err := h.startMember()
				if err != nil {
					t.Logf("[chaos %s] kill-restart m%d -> start FAILED: %v", ts(), victim.id, err)
					continue
				}
				t.Logf("[chaos %s] kill-restart m%d -> m%d (count=%d)", ts(), victim.id, m.id, h.memberCount())
			case 3: // orphan injection: directly write a synthetic orphan record
				// Rotate among the three shapes so each is exercised.
				shape := orphanShape(orphanSeq % 3)
				orphanSeq++
				o := h.injectOrphan(rng, shape, orphanSeq)
				if o != nil {
					t.Logf("[chaos %s] inject orphan key=%s shape=%s", ts(), o.key, o.shape)
				} else {
					t.Logf("[chaos %s] inject orphan skipped (no live member or no dead member yet)", ts())
				}
			}
		}
	}()

	// --- I1 checker: poll the registry violation flag ---
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				if v := soakRegistry.firstViolation(); v != nil {
					select {
					case fatalCh <- *v:
					default:
					}
					return
				}
			}
		}
	}()

	// --- I2 checker: per-identity nil span while continuously alive ---
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				key, span := spans.worstSpan()
				if span > wedgeThreshold && h.aliveContinuouslyFor(span) {
					msg := fmt.Sprintf(
						"I2 PERMANENT WEDGE: identity %q returned nil for %s (threshold %s) "+
							"while >=1 member was continuously alive the whole span. "+
							"member count now=%d. This exceeds the branch's bounded worst-case "+
							"(~45s activation + 60s reap + 30s janitor).",
						key, span.Round(time.Second), wedgeThreshold, h.memberCount())
					select {
					case fatalCh <- msg:
					default:
					}
					return
				}
			}
		}
	}()

	// --- I3 checker: orphan lock-only records + count bound; also I4 ---
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(orphanCheckInterval)
		defer ticker.Stop()
		// maxRecords: load-loop identities x2 + injected orphan budget + margin.
		// Injected orphans add at most ~1 per chaos tick (every 2-20s); at the
		// default 10m run pace that is at most ~300 orphans in flight before any
		// cleanup happens, but cleanup is fast (30-60s), so a steady-state bound
		// of 30 on top of the load-loop identities is generous.
		maxRecords := len(ids)*2 + 30 + 8
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				// I3: check native orphans in the bucket.
				if msg := checkOrphans(h, maxRecords); msg != "" {
					select {
					case fatalCh <- msg:
					default:
					}
					return
				}
				// I4: every injected orphan must be gone within orphanCleanDeadline.
				if msg := checkInjectedOrphans(h); msg != "" {
					select {
					case fatalCh <- msg:
					default:
					}
					return
				}
			}
		}
	}()

	// --- prober: periodically Get each pending injected shape-(a)/(b) orphan ---
	// This exercises the reap-on-Get path in addition to the janitor sweep.
	// Shape-(c) orphans use soakOrphanKind which no member serves, so Getting
	// them would only ever return nil and would not trigger the reap-on-Get path
	// (they need the janitor). We skip them here intentionally.
	wg.Add(1)
	go func() {
		defer wg.Done()
		prng := rand.New(rand.NewSource(seed + int64(soakLoadLoops) + 99))
		for {
			select {
			case <-stop:
				return
			default:
			}
			sleep := proberMinInterval + time.Duration(prng.Int63n(int64(proberMaxInterval-proberMinInterval)))
			select {
			case <-stop:
				return
			case <-time.After(sleep):
			}

			// Snapshot pending orphans that are lock-only shapes (a) and (b).
			pending := soakOrphanReg.snapshot()
			live := h.liveMembers()
			if len(live) == 0 || len(pending) == 0 {
				continue
			}
			m := live[prng.Intn(len(live))]

			for _, o := range pending {
				if o.cleanedT.IsZero() && (o.shape == shapeDeadOwnerLock || o.shape == shapeLegacyLock) {
					// Build a ClusterIdentity matching the injected key.
					// Key format: "soakorphan/<identity-string>".
					// We cannot call cluster.Get for soakorphan because it is not
					// a registered Kind; use the IdentityLookup directly to trigger
					// the reap-on-Get path which only needs a NATS KV read.
					// We read the raw entry to confirm the record is still present;
					// if absent, mark it cleaned.
					il := m.provider.IdentityLookup()
					if il == nil {
						continue
					}
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					entry, err := il.identities.Get(ctx, o.key)
					cancel()
					if err != nil {
						// Key is gone -- mark cleaned.
						soakOrphanReg.markCleaned(o.key, time.Now())
					} else {
						// Key still present. Invoke maybeReapLock via a bare
						// ClusterIdentity so the reap-on-Get path fires for shapes
						// (a)/(b). The CI kind/identity are not checked inside
						// maybeReapLock -- only the KV record contents matter.
						var rec activationRecord
						if json.Unmarshal(entry.Value(), &rec) == nil && rec.PidID == "" {
							// Lock-only record: eligible for maybeReapLock.
							ci := &cluster.ClusterIdentity{
								Kind:     soakOrphanKind,
								Identity: o.key[len(soakOrphanKind)+1:], // strip "soakorphan/"
							}
							il.maybeReapLock(context.Background(), ci)
						}
					}
				}
			}
		}
	}()

	// --- main wait loop: end on deadline or first fatal ---
	var violation string
	overall := time.NewTimer(time.Until(deadline))
	defer overall.Stop()
	progress := time.NewTicker(30 * time.Second)
	defer progress.Stop()

waitLoop:
	for {
		select {
		case violation = <-fatalCh:
			break waitLoop
		case <-overall.C:
			break waitLoop
		case <-progress.C:
			p50, p99, mx := stats.percentiles()
			t.Logf("[progress %s] elapsed=%s gets=%d nils=%d members=%d p50=%s p99=%s max=%s reap=%d janitor=%d",
				ts(), time.Since(startedAt).Round(time.Second),
				stats.total.Load(), stats.nils.Load(), h.memberCount(),
				p50.Round(time.Millisecond), p99.Round(time.Millisecond), mx.Round(time.Millisecond),
				handler.reap.Load(), handler.janitor.Load())
			logResourceTelemetry(t, h, startedAt)
		}
	}

	// Stop all background goroutines.
	close(stop)
	wg.Wait()

	// Drain any late violation that raced with shutdown.
	if violation == "" {
		select {
		case violation = <-fatalCh:
		default:
		}
	}

	// Capture final bucket state while members are still alive (their
	// IdentityLookup handles read the shared identities bucket).
	finalCount, lockOnly := finalBucketState(h)

	// Tear down remaining members.
	for _, m := range h.liveMembers() {
		h.stopMember(m, true)
	}

	// --- end-of-run report ---
	total := stats.total.Load()
	nils := stats.nils.Load()
	nilRate := 0.0
	if total > 0 {
		nilRate = float64(nils) / float64(total) * 100
	}
	p50, p99, mx := stats.percentiles()

	injByShape, cleanByShape, maxTTC := soakOrphanReg.totals()
	totalInjected := injByShape[0] + injByShape[1] + injByShape[2]
	totalCleaned := cleanByShape[0] + cleanByShape[1] + cleanByShape[2]
	reapCnt := handler.reap.Load()
	janitorCnt := handler.janitor.Load()
	cleanCnt := handler.clean.Load()

	t.Logf("=== SOAK REPORT ===")
	t.Logf("seed:            %d", seed)
	t.Logf("duration:        %s (requested %s)", time.Since(startedAt).Round(time.Second), duration)
	t.Logf("total Gets:      %d", total)
	t.Logf("nil results:     %d (%.2f%%)", nils, nilRate)
	t.Logf("Get p50/p99/max: %s / %s / %s",
		p50.Round(time.Millisecond), p99.Round(time.Millisecond), mx.Round(time.Millisecond))
	t.Logf("reap log count:  %d", reapCnt)
	t.Logf("janitor count:   %d", janitorCnt)
	t.Logf("stale-clean cnt: %d", cleanCnt)
	t.Logf("final records:   %d (lock-only: %d)", finalCount, lockOnly)
	t.Logf("--- orphan injection ---")
	t.Logf("injected:        %d total (dead-owner-lock=%d legacy-lock=%d dead-activation=%d)",
		totalInjected, injByShape[0], injByShape[1], injByShape[2])
	t.Logf("cleaned:         %d total (dead-owner-lock=%d legacy-lock=%d dead-activation=%d)",
		totalCleaned, cleanByShape[0], cleanByShape[1], cleanByShape[2])
	t.Logf("max time-to-clean: %s", maxTTC.Round(time.Millisecond))

	if violation != "" {
		t.Fatalf("INVARIANT VIOLATION:\n%s", violation)
	}

	// End-of-run assertion: if any orphans were injected, counters must be nonzero.
	// A zero counter with injections means no cleanup path fired at all -- the run
	// is inconclusive (it did not exercise the mechanisms it was designed to test).
	if totalInjected > 0 {
		allCounters := reapCnt + janitorCnt + cleanCnt
		if allCounters == 0 {
			t.Fatalf("INCONCLUSIVE: %d orphans were injected but reap+janitor+stale-clean counters are all zero. "+
				"No cleanup path fired. Verify log-scrape patterns match actual log messages.",
				totalInjected)
		}
	}

	t.Logf("PASS seed=%d", seed)
}

// safeGet wraps cluster.Get so a panic inside a mid-shutdown member (racing the
// chaos loop) surfaces as a nil result rather than crashing the load loop. A
// real double-activation is caught by I1, not by masking here; this only guards
// the inherent races of yanking a member out from under an in-flight Get.
func safeGet(c *cluster.Cluster, ci *cluster.ClusterIdentity) (pid *actor.PID) {
	defer func() {
		if recover() != nil {
			pid = nil
		}
	}()
	return c.Get(ci.Identity, ci.Kind)
}

// checkOrphans inspects the identities bucket for I3: any lock-only record
// (PidID == "") older than orphanThreshold is a violation, and the total record
// count must not exceed maxRecords. Returns a non-empty message on violation.
//
// Injected orphan keys (soakOrphanKind prefix) are excluded from the I3
// orphanThreshold check because they are governed by the wider I4 window
// (orphanCleanDeadline). They ARE counted toward maxRecords.
// While scanning, any injected key that is now absent is marked cleaned.
func checkOrphans(h *soakHarness, maxRecords int) string {
	il := h.anyLiveIdentityLookup()
	if il == nil {
		return "" // cluster momentarily empty; skip this sweep
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	keys, err := il.identities.Keys(ctx)
	if err != nil {
		// A transient KV error during churn is not a violation.
		return ""
	}

	// Build a set of injected keys still pending, so we can mark cleaned ones.
	pendingInjected := make(map[string]*injectedOrphan)
	for _, o := range soakOrphanReg.snapshot() {
		if o.cleanedT.IsZero() {
			pendingInjected[o.key] = o
		}
	}
	// Keys present in bucket: collect.
	presentKeys := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		presentKeys[k] = struct{}{}
	}
	// Any pending injected key no longer in the bucket is now cleaned.
	now := time.Now()
	for k := range pendingInjected {
		if _, stillPresent := presentKeys[k]; !stillPresent {
			soakOrphanReg.markCleaned(k, now)
		}
	}

	lockOnly := 0
	for _, key := range keys {
		entry, err := il.identities.Get(ctx, key)
		if err != nil {
			continue
		}
		var rec activationRecord
		if json.Unmarshal(entry.Value(), &rec) != nil {
			continue
		}
		if rec.PidID == "" {
			lockOnly++
			// Injected orphans use the wider I4 deadline; skip the I3 threshold.
			if _, isInjected := pendingInjected[key]; isInjected {
				continue
			}
			age := entryAge(entry, now)
			if age > orphanThreshold {
				return fmt.Sprintf(
					"I3 ETERNAL ORPHAN: lock-only record %q (member=%s lock=%s) has age %s "+
						"(threshold %s). The reap/janitor path failed to clear an "+
						"abandoned lock. members alive=%d",
					key, rec.MemberID, rec.LockID, age.Round(time.Second), orphanThreshold, h.memberCount())
			}
		}
	}

	if len(keys) > maxRecords {
		return fmt.Sprintf(
			"I3 RECORD LEAK: identities bucket holds %d records (bound %d = identities x2 + injection-budget + margin). "+
				"lock-only=%d. Records are accumulating faster than they are reaped. members alive=%d",
			len(keys), maxRecords, lockOnly, h.memberCount())
	}
	return ""
}

// checkInjectedOrphans implements I4: every injected orphan must be absent from
// the identities bucket within orphanCleanDeadline of injection. Returns a
// non-empty failure message if any survivor is found past the deadline.
func checkInjectedOrphans(h *soakHarness) string {
	il := h.anyLiveIdentityLookup()
	if il == nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	now := time.Now()
	for _, o := range soakOrphanReg.snapshot() {
		if !o.cleanedT.IsZero() {
			continue // already confirmed cleaned
		}
		age := now.Sub(o.injectedT)
		if age < orphanCleanDeadline {
			continue // still within grace window
		}
		// Past deadline: check whether the key is still in the bucket.
		_, err := il.identities.Get(ctx, o.key)
		if err != nil {
			// Key absent: mark cleaned retroactively.
			soakOrphanReg.markCleaned(o.key, now)
			continue
		}
		// Key is still present past deadline: I4 violation.
		return fmt.Sprintf(
			"I4 INJECTED ORPHAN SURVIVOR: key %q (shape=%s injected=%s) has NOT been "+
				"cleaned %s after injection (deadline %s). "+
				"The reap/janitor/grace path failed to clear this synthetic orphan. "+
				"members alive=%d",
			o.key, o.shape,
			o.injectedT.Format("15:04:05"),
			age.Round(time.Second),
			orphanCleanDeadline,
			h.memberCount())
	}
	return ""
}

// finalBucketState returns the final identity record count and lock-only count.
// It is called before the last members are torn down, so a live member's
// IdentityLookup handle is used to read the shared identities bucket.
func finalBucketState(h *soakHarness) (total, lockOnly int) {
	il := h.anyLiveIdentityLookup()
	if il == nil {
		return 0, 0
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	keys, err := il.identities.Keys(ctx)
	if err != nil {
		return -1, -1
	}
	for _, key := range keys {
		entry, err := il.identities.Get(ctx, key)
		if err != nil {
			continue
		}
		var rec activationRecord
		if json.Unmarshal(entry.Value(), &rec) != nil {
			continue
		}
		total++
		if rec.PidID == "" {
			lockOnly++
		}
	}
	return total, lockOnly
}

// --- env helpers ------------------------------------------------------------

func envDuration(name string, def time.Duration) time.Duration {
	v := os.Getenv(name)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}

func envInt(name string, def int) int {
	v := os.Getenv(name)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

// envSeed reads SOAK_SEED or derives a seed from the current time.
func envSeed(name string) int64 {
	v := os.Getenv(name)
	if v == "" {
		return time.Now().UnixNano()
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return time.Now().UnixNano()
	}
	return n
}

// ts returns a compact timestamp for chaos/progress logs.
func ts() string {
	return time.Now().Format("15:04:05.000")
}

// logResourceTelemetry emits a periodic resource line so a multi-hour run can be
// diagnosed for growth. It reports runtime.MemStats (HeapAlloc / HeapInuse /
// Sys), the live goroutine count, the current live member count, and the
// cumulative members started/stopped. The diagnostic reading:
//
//   - goroutines climbing in lock-step with `stopped` == leaked per-member
//     teardown (a Shutdown path that fails to release goroutines/connections).
//   - HeapAlloc climbing while goroutines stay flat == data accumulation
//     (e.g. the embedded JetStream store or unbounded harness bookkeeping).
//   - both flat == the working set is stable; any OOM is a legitimate
//     steady-state requirement, not a leak.
func logResourceTelemetry(t *testing.T, h *soakHarness, startedAt time.Time) {
	t.Helper()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	t.Logf("[resource %s] elapsed=%s goroutines=%d heapAlloc=%dMiB heapInuse=%dMiB sys=%dMiB "+
		"members=%d started=%d stopped=%d",
		ts(), time.Since(startedAt).Round(time.Second),
		runtime.NumGoroutine(),
		ms.HeapAlloc/(1024*1024), ms.HeapInuse/(1024*1024), ms.Sys/(1024*1024),
		h.memberCount(), h.started.Load(), h.stopped.Load())
}
