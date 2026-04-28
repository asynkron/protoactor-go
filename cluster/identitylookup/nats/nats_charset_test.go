package nats

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/require"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/cluster"
)

// charsetCase is a single character-set probe for the standalone NATS
// identity storage backend.
type charsetCase struct {
	name  string
	value string
}

// charsetProbeCases mirrors the cases in the integrated natskv suite so the
// two reports can be diffed side-by-side. Keep in sync.
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
		{"emoji", "rocket-\xf0\x9f\x9a\x80"},
		{"accent", "café"},
		{"cjk", "日本語"},
		{"cyrillic", "Привет"},

		// Edge sizes.
		{"empty", ""},
		{"long_300", strings.Repeat("a", 300)},
	}
}

// startEmbeddedNATSCharset starts an embedded NATS server with JetStream and
// returns the JetStream context. Used so this test does not require a
// running NATS container.
func startEmbeddedNATSCharset(t *testing.T) jetstream.JetStream {
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

	nc, err := nats.Connect(srv.ClientURL())
	require.NoError(t, err)
	t.Cleanup(func() { nc.Close() })

	js, err := jetstream.New(nc)
	require.NoError(t, err)
	return js
}

// storageProbeOutcome records what the storage layer did for a given case.
type storageProbeOutcome struct {
	keyMapped     string // result of kvKey(ci)
	rawCreateErr  error  // result of identities.Create using the mapped key
	lockAcquired  bool   // TryAcquireLock returned a non-nil SpawnLock
	storeOK       bool   // StoreActivation followed by TryGetExistingActivation found the record
	getActivation *cluster.StoredActivation
}

// probeStorage exercises every public storage operation that depends on the
// computed key. It returns enough detail for the test to print a row and for
// callers to see at which layer a value fails.
func probeStorage(t *testing.T, s *NatsIdentityStorage, ci *cluster.ClusterIdentity) storageProbeOutcome {
	t.Helper()
	out := storageProbeOutcome{keyMapped: kvKey(ci)}

	ctx := context.Background()

	// Direct Create surfaces NATS-side validation errors that the storage
	// layer otherwise swallows.
	_, out.rawCreateErr = s.identities.Create(ctx, out.keyMapped, []byte(`{}`))
	if out.rawCreateErr == nil {
		// Clean up so the storage-layer call below has a fresh slate.
		_ = s.identities.Delete(ctx, out.keyMapped)
	}

	lock := s.TryAcquireLock(ci)
	if lock == nil {
		return out
	}
	out.lockAcquired = true

	pid := actor.NewPID("127.0.0.1:1234", "echo/probe")
	s.StoreActivation("test-member", lock, pid)

	out.getActivation = s.TryGetExistingActivation(ci)
	if out.getActivation != nil {
		out.storeOK = true
	}

	// Best-effort cleanup so subsequent cases don't see leftover state if a
	// future case happens to share a key (none do today, but cheap insurance).
	s.RemoveActivation(&cluster.SpawnLock{LockID: lock.LockID, ClusterIdentity: ci})

	return out
}

// logOutcome prints a single row of the report. The format is intentionally
// dense so the captured output is the deliverable for "what works".
func logOutcome(t *testing.T, label string, out storageProbeOutcome) {
	t.Helper()
	switch {
	case out.storeOK:
		t.Logf("%s: OK key=%q", label, out.keyMapped)
	case out.lockAcquired:
		t.Logf("%s: LOCK_OK_BUT_STORE_FAILED key=%q", label, out.keyMapped)
	case out.rawCreateErr != nil && errors.Is(out.rawCreateErr, jetstream.ErrInvalidKey):
		t.Logf("%s: REJECTED_INVALID_KEY key=%q", label, out.keyMapped)
	case out.rawCreateErr != nil:
		t.Logf("%s: NATS_ERROR key=%q err=%v", label, out.keyMapped, out.rawCreateErr)
	default:
		t.Logf("%s: UNEXPECTED key=%q outcome=%+v", label, out.keyMapped, out)
	}
}

// TestCharset_NatsIdentity_Identity probes Identity character sets against
// the standalone NatsIdentityStorage (NOT the integrated natskv variant).
// Kind is held fixed at "echo" so the only variable is the identity string.
func TestCharset_NatsIdentity_Identity(t *testing.T) {
	js := startEmbeddedNATSCharset(t)
	storage, err := New("charset_id", js)
	require.NoError(t, err)
	t.Cleanup(func() {
		ctx := context.Background()
		_ = js.DeleteKeyValue(ctx, "charset_id_identities")
		_ = js.DeleteKeyValue(ctx, "charset_id_members")
	})

	for _, tc := range charsetProbeCases() {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if err := cluster.ValidateIdentity(tc.value); err != nil {
				ci := cluster.NewClusterIdentity(tc.value, "echo")
				require.Nil(t, storage.TryAcquireLock(ci),
					"storage must reject invalid identity %q before hitting NATS", tc.value)
				t.Logf("identity=%s: REJECTED_BY_VALIDATION err=%v", tc.name, err)
				return
			}
			ci := cluster.NewClusterIdentity(tc.value, "echo")
			out := probeStorage(t, storage, ci)
			logOutcome(t, "identity="+tc.name, out)
		})
	}
}

// TestCharset_NatsIdentity_IdentityValidation asserts that the standalone
// storage backend short-circuits invalid identities before issuing a NATS
// request. Mirrors TestCharset_NatsKV_IdentityValidation in the natskv
// package.
func TestCharset_NatsIdentity_IdentityValidation(t *testing.T) {
	js := startEmbeddedNATSCharset(t)
	storage, err := New("charset_id_validation", js)
	require.NoError(t, err)
	t.Cleanup(func() {
		ctx := context.Background()
		_ = js.DeleteKeyValue(ctx, "charset_id_validation_identities")
		_ = js.DeleteKeyValue(ctx, "charset_id_validation_members")
	})

	ci := cluster.NewClusterIdentity("", "MyKind")
	require.Nil(t, storage.TryAcquireLock(ci),
		"empty identity must be rejected at the storage layer")
	require.Nil(t, storage.TryGetExistingActivation(ci),
		"TryGetExistingActivation must short-circuit empty identities")
}

// TestCharset_NatsIdentity_Kind probes Kind character sets against the same
// standalone backend. Identity is held fixed at "user1".
func TestCharset_NatsIdentity_Kind(t *testing.T) {
	js := startEmbeddedNATSCharset(t)
	storage, err := New("charset_kind", js)
	require.NoError(t, err)
	t.Cleanup(func() {
		ctx := context.Background()
		_ = js.DeleteKeyValue(ctx, "charset_kind_identities")
		_ = js.DeleteKeyValue(ctx, "charset_kind_members")
	})

	for _, tc := range charsetProbeCases() {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ci := cluster.NewClusterIdentity("user1", tc.value)
			out := probeStorage(t, storage, ci)
			logOutcome(t, "kind="+tc.name, out)
		})
	}
}

// TestCharset_NatsIdentity_kvKeyMapping documents what kvKey produces for
// each probe value in this package. After the rewrite changes, kvKey only
// substitutes ':' -> '_' and preserves '/' verbatim, matching the natskv
// integrated lookup's behaviour. The mapping should be identical between the
// two packages.
func TestCharset_NatsIdentity_kvKeyMapping(t *testing.T) {
	for _, tc := range charsetProbeCases() {
		tc := tc
		t.Run("identity_"+tc.name, func(t *testing.T) {
			ci := cluster.NewClusterIdentity(tc.value, "echo")
			t.Logf("identity=%q -> kvKey=%q", tc.value, kvKey(ci))
		})
		t.Run("kind_"+tc.name, func(t *testing.T) {
			ci := cluster.NewClusterIdentity("user1", tc.value)
			t.Logf("kind=%q -> kvKey=%q", tc.value, kvKey(ci))
		})
	}
}

// TestCharset_NatsIdentity_SlashIdentityRoundTrip verifies that an identity
// containing '/' characters round-trips through StoreActivation /
// ListActivationsByMember with the original slashes intact. This is the
// storage-layer analogue of the natskv full-cluster round-trip test.
func TestCharset_NatsIdentity_SlashIdentityRoundTrip(t *testing.T) {
	js := startEmbeddedNATSCharset(t)
	storage, err := New("charset_slash_rt", js)
	require.NoError(t, err)
	t.Cleanup(func() {
		ctx := context.Background()
		_ = js.DeleteKeyValue(ctx, "charset_slash_rt_identities")
		_ = js.DeleteKeyValue(ctx, "charset_slash_rt_members")
	})

	const memberID = "test-member"
	const kind = "MyKind"
	const identity = "org/team/user-42"

	ci := cluster.NewClusterIdentity(identity, kind)
	lock := storage.TryAcquireLock(ci)
	require.NotNil(t, lock, "lock should be acquired for slash identity")

	pid := actor.NewPID("127.0.0.1:1234", "MyKind/"+identity)
	storage.StoreActivation(memberID, lock, pid)

	infos, err := storage.ListActivationsByMember(memberID)
	require.NoError(t, err)
	require.Len(t, infos, 1, "exactly one activation should be tracked")

	got := infos[0]
	require.Equal(t, kind, got.Kind, "kind should be parsed from before the first '/'")
	require.Equal(t, identity, got.Identity, "identity should preserve internal '/' characters")
	t.Logf("round-trip OK: kind=%q identity=%q", got.Kind, got.Identity)
}
