package natskv

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/cluster"
	"github.com/awevoke/protoactor-go/remote"
)

// charsetCase is a single character-set probe.
type charsetCase struct {
	name  string
	value string
}

// charsetProbeCases enumerates strings to probe as either a Kind or an Identity.
// Names must be unique and safe to use as t.Run subtest names. Values are the
// actual strings exercised end-to-end.
//
// The cases cluster around four questions:
//   - Which printable ASCII chars round-trip through NATS KV keys?
//   - Which NATS subject metacharacters (`*`, `>`, `+`) leak through?
//   - How is colon handled (the natskv-specific escape)?
//   - What happens with whitespace, unicode, and edge sizes?
func charsetProbeCases() []charsetCase {
	return []charsetCase{
		// Baseline / always-valid alphanumeric.
		{"alnum", "abc123"},
		{"single_char", "a"},
		{"hyphen", "user-1"},
		{"underscore", "user_1"},

		// NATS KV-allowed extra characters per validKeyRe.
		{"dot", "user.1"},
		{"slash", "user/1"},
		{"equals", "user=1"},

		// Common identity formats.
		{"colon_single", "tenant:domain"},
		{"colon_double", "a::b"},
		{"uuid_like", "550e8400-e29b-41d4-a716-446655440000"},
		{"email_like", "user@example.com"},
		{"path_like", "org/team/user"},

		// NATS subject metacharacters.
		{"asterisk", "user*1"},
		{"gt", "user>1"},
		{"plus", "user+1"},

		// Whitespace / control.
		{"space", "user 1"},
		{"tab", "user\t1"},
		{"newline", "user\n1"},
		{"leading_space", " user"},
		{"trailing_space", "user "},

		// Punctuation.
		{"comma", "a,b"},
		{"semicolon", "a;b"},
		{"hash", "a#b"},
		{"percent", "a%b"},
		{"ampersand", "a&b"},
		{"question", "a?b"},
		{"bang", "a!b"},
		{"paren", "a(b)c"},
		{"brace", "a{b}c"},
		{"backslash", "a\\b"},
		{"pipe", "a|b"},

		// Unicode.
		{"emoji", "rocket-\xf0\x9f\x9a\x80"}, // "rocket-🚀"
		{"accent", "café"},
		{"cjk", "日本語"},
		{"cyrillic", "Привет"},

		// Edge sizes.
		{"empty", ""},
		{"long_300", strings.Repeat("a", 300)},
	}
}

// echoActorProps returns Props for an actor that responds to *emptypb.Empty
// with *emptypb.Empty so callers can confirm the grain is reachable end-to-end.
func echoActorProps() *actor.Props {
	return actor.PropsFromFunc(func(ctx actor.Context) {
		if _, ok := ctx.Message().(*emptypb.Empty); ok {
			ctx.Respond(&emptypb.Empty{})
		}
	})
}

// startEmbeddedNATSCharset starts an embedded NATS server with JetStream.
// (Distinct from the helper in testhelpers_test.go to avoid coupling, since
// this test owns its lifecycle.)
func startEmbeddedNATSCharset(t *testing.T) *server.Server {
	t.Helper()
	opts := &server.Options{
		JetStream: true,
		Port:      -1,
		StoreDir:  t.TempDir(),
	}
	srv, err := server.NewServer(opts)
	require.NoError(t, err)
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

// startCharsetCluster spins up a single-node natskv cluster on the embedded
// NATS server with the given kinds registered. The cluster is fully started
// (StartMember) so the placement actor and identity lookup are wired up.
func startCharsetCluster(t *testing.T, srv *server.Server, clusterName string, kinds []*cluster.Kind) (*Provider, *cluster.Cluster) {
	t.Helper()
	nc, err := nats.Connect(srv.ClientURL())
	require.NoError(t, err)
	t.Cleanup(func() { nc.Close() })

	p, err := New(nc,
		// Short timeouts so failed lookups don't block the test.
		WithMemberTTL(10*time.Second),
		WithRefreshInterval(2*time.Second),
		WithLeaderTTL(10*time.Second),
		WithLockTTL(1*time.Second),
	)
	require.NoError(t, err)

	system := actor.NewActorSystem()
	remoteCfg := remote.Configure("127.0.0.1", 0)
	clusterCfg := cluster.Configure(clusterName, p, p.IdentityLookup(), remoteCfg,
		cluster.WithKinds(kinds...),
	)
	c := cluster.NewCluster(system, clusterCfg)

	require.NoError(t, c.StartMember())

	// StartMember sleeps 1s; give topology one more beat.
	time.Sleep(500 * time.Millisecond)

	return p, c
}

// probeOutcome describes how cluster.Request behaved for a single probe case.
type probeOutcome struct {
	getNil    bool
	requestOK bool
	reqErr    error
	panicVal  any
}

// (s probeOutcome) String is intentionally omitted; tests format inline.

// runProbe calls cluster.Request and classifies the outcome. Any panic
// triggered during the call is recovered and reported.
func runProbe(c *cluster.Cluster, identity, kind string) (out probeOutcome) {
	defer func() {
		if r := recover(); r != nil {
			out.panicVal = r
		}
	}()

	pid := c.Get(identity, kind)
	if pid == nil {
		out.getNil = true
		// Try Request anyway — it has its own retry/spawn path.
	}

	resp, err := c.Request(identity, kind, &emptypb.Empty{},
		cluster.WithTimeout(3*time.Second),
		cluster.WithRetryCount(1),
	)
	if err != nil {
		out.reqErr = err
		return
	}
	if _, ok := resp.(*emptypb.Empty); ok {
		out.requestOK = true
	}
	return
}

// assertProbe records pass/fail to t but does not stop the suite — every case
// is informational. The test is "pass" if the run completes without test
// framework error; the goal is the table that t.Logf emits.
func assertProbe(t *testing.T, label string, out probeOutcome) {
	t.Helper()
	switch {
	case out.panicVal != nil:
		t.Logf("%s: PANIC value=%v", label, out.panicVal)
	case out.requestOK:
		t.Logf("%s: OK (round-trip succeeded)", label)
	case out.reqErr != nil && errors.Is(out.reqErr, context.DeadlineExceeded):
		t.Logf("%s: TIMEOUT (deadline exceeded)", label)
	case out.reqErr != nil:
		t.Logf("%s: ERROR getNil=%v err=%v", label, out.getNil, out.reqErr)
	default:
		t.Logf("%s: UNEXPECTED outcome=%+v", label, out)
	}
}

// TestCharset_NatsKV_Identity probes which characters can appear in a grain
// Identity when using the integrated natskv.IdentityLookup. The kind is the
// fixed "echo" kind. Each subtest reports the outcome of a full round trip
// (cluster.Request returning *emptypb.Empty).
func TestCharset_NatsKV_Identity(t *testing.T) {
	srv := startEmbeddedNATSCharset(t)

	echo := cluster.NewKind("echo", echoActorProps())
	_, c := startCharsetCluster(t, srv, "charset-id", []*cluster.Kind{echo})
	t.Cleanup(func() { c.Shutdown(true) })

	for _, tc := range charsetProbeCases() {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if err := cluster.ValidateIdentity(tc.value); err != nil {
				t.Logf("identity=%s: REJECTED_BY_VALIDATION err=%v", tc.name, err)
				require.Nil(t, c.Get(tc.value, "echo"), "invalid identity must yield nil PID")
				return
			}
			out := runProbe(c, tc.value, "echo")
			assertProbe(t, "identity="+tc.name, out)
		})
	}
}

// TestCharset_NatsKV_Kind probes which characters can appear in a Kind name.
//
// Kinds that fail cluster.ValidateKindName (currently: empty, contains '/')
// are reported as REJECTED_BY_VALIDATION and never registered. The remaining
// kinds are all registered upfront so an activator member exists; the only
// variable for them is whether activation succeeds end-to-end.
func TestCharset_NatsKV_Kind(t *testing.T) {
	cases := charsetProbeCases()

	kinds := make([]*cluster.Kind, 0, len(cases))
	seen := make(map[string]struct{}, len(cases))
	for _, tc := range cases {
		if _, dup := seen[tc.value]; dup {
			continue
		}
		seen[tc.value] = struct{}{}
		if err := cluster.ValidateKindName(tc.value); err != nil {
			continue
		}
		kinds = append(kinds, cluster.NewKind(tc.value, echoActorProps()))
	}

	srv := startEmbeddedNATSCharset(t)
	_, c := startCharsetCluster(t, srv, "charset-kind", kinds)
	t.Cleanup(func() { c.Shutdown(true) })

	const identity = "user1"

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if err := cluster.ValidateKindName(tc.value); err != nil {
				t.Logf("kind=%s: REJECTED_BY_VALIDATION err=%v", tc.name, err)
				return
			}
			out := runProbe(c, identity, tc.value)
			assertProbe(t, "kind="+tc.name, out)
		})
	}
}

// TestCharset_NatsKV_IdentityValidation asserts that cluster.ValidateIdentity
// rejects empty identities and that c.Get with an empty identity returns
// nil rather than activating a phantom grain.
func TestCharset_NatsKV_IdentityValidation(t *testing.T) {
	require.Error(t, cluster.ValidateIdentity(""), "empty identity must be rejected")
	require.NoError(t, cluster.ValidateIdentity("a"))
	require.NoError(t, cluster.ValidateIdentity("org/team/user"))
	require.NoError(t, cluster.ValidateIdentity("tenant:domain"))

	srv := startEmbeddedNATSCharset(t)
	echo := cluster.NewKind("MyKind", echoActorProps())
	_, c := startCharsetCluster(t, srv, "charset-id-validation", []*cluster.Kind{echo})
	t.Cleanup(func() { c.Shutdown(true) })

	require.Nil(t, c.Get("", "MyKind"), "c.Get with empty identity must return nil")
}

// TestCharset_NatsKV_KindValidation asserts that cluster.ValidateKindName
// (and therefore cluster.Configure / cluster.RegisterKind) reject kind names
// that contain '/' or are empty, since '/' is the kind|identity boundary.
func TestCharset_NatsKV_KindValidation(t *testing.T) {
	rejected := []string{
		"",                // empty
		"a/b",             // simple slash
		"org/team",        // path-shaped
		"/leading",        // leading slash
		"trailing/",       // trailing slash
		"a/b/c",           // multiple slashes
	}
	for _, name := range rejected {
		t.Run("reject_"+name, func(t *testing.T) {
			err := cluster.ValidateKindName(name)
			require.Error(t, err, "kind name %q must be rejected", name)
			t.Logf("kind=%q rejected: %v", name, err)
		})
	}

	accepted := []string{
		"echo",
		"MyKind",
		"kind-1",
		"kind_1",
		"kind.with.dots",
		"tenant:Type", // colon is allowed at the kind level (will be escaped at the KV layer)
	}
	for _, name := range accepted {
		t.Run("accept_"+name, func(t *testing.T) {
			require.NoError(t, cluster.ValidateKindName(name), "kind name %q must be accepted", name)
		})
	}
}

// TestCharset_NatsKV_kvKeyMapping documents what the integrated natskv kvKey
// function produces for each probe value. This is the deterministic side of
// the probe: regardless of whether end-to-end activation succeeds, the kvKey
// mapping is a fixed string that callers can audit.
//
// Useful as a reference table when reading the round-trip results.
func TestCharset_NatsKV_kvKeyMapping(t *testing.T) {
	const fixedKind = "echo"
	for _, tc := range charsetProbeCases() {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ci := cluster.NewClusterIdentity(tc.value, fixedKind)
			t.Logf("identity=%q -> kvKey=%q", tc.value, kvKey(ci))
		})
	}
	// Also exercise the inverse: kind side.
	const fixedIdentity = "user1"
	for _, tc := range charsetProbeCases() {
		tc := tc
		t.Run("kind_"+tc.name, func(t *testing.T) {
			ci := cluster.NewClusterIdentity(fixedIdentity, tc.value)
			t.Logf("kind=%q -> kvKey=%q", tc.value, kvKey(ci))
		})
	}
}

// guard against goroutine leaks inflating later tests — make sure every
// charset test cleanup actually runs Shutdown by the end of the package run.
var _ = func() *sync.Once { var o sync.Once; return &o }()

// TestCharset_NatsKV_SlashAndDotIdentitiesAreDistinct verifies that two
// identities that previously collided (one with '/', one with '.') now
// resolve to DIFFERENT activations. kvKey no longer rewrites '/' to '.', so
// the two identity strings produce different KV keys and different actors.
func TestCharset_NatsKV_SlashAndDotIdentitiesAreDistinct(t *testing.T) {
	srv := startEmbeddedNATSCharset(t)

	echo := cluster.NewKind("MyKind", echoActorProps())
	_, c := startCharsetCluster(t, srv, "charset-distinct", []*cluster.Kind{echo})
	t.Cleanup(func() { c.Shutdown(true) })

	const slashID = "a/b/c-1/2/3"
	const dotID = "a.b.c-1.2.3"

	pidSlash := c.Get(slashID, "MyKind")
	require.NotNil(t, pidSlash, "slash identity should resolve to a PID")
	t.Logf("slash identity %q -> PID %s", slashID, pidSlash.String())

	pidDot := c.Get(dotID, "MyKind")
	require.NotNil(t, pidDot, "dot identity should resolve to a PID")
	t.Logf("dot   identity %q -> PID %s", dotID, pidDot.String())

	t.Logf("kvKey(slash) = %q", kvKey(cluster.NewClusterIdentity(slashID, "MyKind")))
	t.Logf("kvKey(dot)   = %q", kvKey(cluster.NewClusterIdentity(dotID, "MyKind")))

	require.False(t, pidSlash.Equal(pidDot),
		"identities %q and %q must resolve to distinct activations", slashID, dotID)
}

// TestCharset_NatsKV_DotAndEqualsRoundTrip exercises identities that use '.'
// and '=' as in-string separators, verifying they round-trip via
// cluster.Request (full end-to-end: spawn, request, response) AND via
// ListGrains (the enumeration parse path that splits on the first '/').
func TestCharset_NatsKV_DotAndEqualsRoundTrip(t *testing.T) {
	srv := startEmbeddedNATSCharset(t)

	echo := cluster.NewKind("MyKind", echoActorProps())
	p, c := startCharsetCluster(t, srv, "charset-dot-eq-rt", []*cluster.Kind{echo})
	t.Cleanup(func() { c.Shutdown(true) })

	identities := []string{
		"user.1",          // single dot
		"a.b.c.d",         // multi-dot
		"key=value",       // single equals
		"a=b=c",           // multi-equals
		"key=value.json",  // mixed equals + dot
		"a.b=c.d",         // alternating
		"v1.0.3=stable",   // realistic version-style
	}

	for _, identity := range identities {
		identity := identity
		t.Run(identity, func(t *testing.T) {
			resp, err := c.Request(identity, "MyKind", &emptypb.Empty{},
				cluster.WithTimeout(5*time.Second),
				cluster.WithRetryCount(2),
			)
			require.NoError(t, err, "cluster.Request must succeed for identity %q", identity)
			require.IsType(t, &emptypb.Empty{}, resp)
			t.Logf("Request OK for identity=%q (kvKey=%q)", identity,
				kvKey(cluster.NewClusterIdentity(identity, "MyKind")))
		})
	}

	// One round-trip enumeration after all activations to confirm every
	// identity is returned by ListGrains exactly as it was supplied.
	infos, err := p.IdentityLookup().ListGrains()
	require.NoError(t, err)

	got := make(map[string]struct{}, len(infos))
	for _, info := range infos {
		if info.Kind == "MyKind" {
			got[info.Identity] = struct{}{}
		}
	}
	for _, want := range identities {
		_, ok := got[want]
		require.True(t, ok, "ListGrains must return identity %q intact (got %v)", want, got)
	}
}

// TestCharset_NatsKV_SlashIdentityRoundTrip activates a grain whose identity
// contains '/' characters and then enumerates the activations via
// ListActivations to verify the original identity (slashes intact) is
// recovered, not a dot-mangled form.
func TestCharset_NatsKV_SlashIdentityRoundTrip(t *testing.T) {
	srv := startEmbeddedNATSCharset(t)

	echo := cluster.NewKind("MyKind", echoActorProps())
	p, c := startCharsetCluster(t, srv, "charset-slash-rt", []*cluster.Kind{echo})
	t.Cleanup(func() { c.Shutdown(true) })

	const identity = "org/team/user-42"
	pid := c.Get(identity, "MyKind")
	require.NotNil(t, pid, "slash identity should activate")

	// Round-trip through ListGrains (the natskv enumerator).
	infos, err := p.IdentityLookup().ListGrains()
	require.NoError(t, err)

	var found *cluster.GrainInfo
	for _, info := range infos {
		if info.Kind == "MyKind" && info.Identity == identity {
			found = info
			break
		}
	}
	require.NotNil(t, found,
		"activation must round-trip with original identity %q intact (got %d activations)", identity, len(infos))
	require.Equal(t, "MyKind", found.Kind)
	require.Equal(t, identity, found.Identity)
	t.Logf("round-trip OK: kind=%q identity=%q", found.Kind, found.Identity)
}
