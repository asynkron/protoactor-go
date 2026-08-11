package natskv

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/awevoke/protoactor-go/cluster"
	"github.com/nats-io/nats.go/jetstream"
)

// testCI constructs a ClusterIdentity for use in unit tests.
func testCI(kind, identity string) *cluster.ClusterIdentity {
	return &cluster.ClusterIdentity{Kind: kind, Identity: identity}
}

// buildBareIdentityLookup creates a bare IdentityLookup with real NATS KV
// buckets but no actor system or placement actor. This pattern mirrors
// stale_cleanup_test.go:105-111 and is suitable for KV-level unit tests.
func buildBareIdentityLookup(t *testing.T) *IdentityLookup {
	t.Helper()

	srv := startEmbeddedNATS(t)
	_, js := connectNATS(t, srv)

	ctx := context.Background()
	identities, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: "test_bare_il_identities_" + t.Name(),
	})
	if err != nil {
		t.Fatalf("buildBareIdentityLookup: create identities bucket: %v", err)
	}

	tracking, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: "test_bare_il_tracking_" + t.Name(),
	})
	if err != nil {
		t.Fatalf("buildBareIdentityLookup: create tracking bucket: %v", err)
	}

	il := &IdentityLookup{
		identities:    identities,
		memberTracker: tracking,
		config:        newDefaultConfig(),
		semaphore:     make(chan struct{}, 200),
		memberID:      "test-member-bare",
		now:           time.Now,
	}
	return il
}

// fakeEntry is a minimal implementation of jetstream.KeyValueEntry
// for use in entryAge tests. Only Created() is meaningful; all other
// methods return zero values.
type fakeEntry struct {
	created time.Time
}

func (f fakeEntry) Bucket() string                  { return "" }
func (f fakeEntry) Key() string                     { return "" }
func (f fakeEntry) Value() []byte                   { return nil }
func (f fakeEntry) Revision() uint64                { return 0 }
func (f fakeEntry) Created() time.Time              { return f.created }
func (f fakeEntry) Delta() uint64                   { return 0 }
func (f fakeEntry) Operation() jetstream.KeyValueOp { return jetstream.KeyValuePut }

func TestReapConfigDefaults(t *testing.T) {
	c := newDefaultConfig()
	for _, tc := range []struct {
		name      string
		got, want time.Duration
	}{
		{"LockOwnerAbsentGrace", c.LockOwnerAbsentGrace, 30 * time.Second},
		{"HardReapAge", c.HardReapAge, 60 * time.Second},
		{"ActivationAbsentGrace", c.ActivationAbsentGrace, 60 * time.Second},
		{"WaiterWindow", c.WaiterWindow, 15 * time.Second},
		{"JanitorInterval", c.JanitorInterval, 30 * time.Second},
		{"LockTTLUnchanged", c.LockTTL, 5 * time.Second},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %v, want %v", tc.name, tc.got, tc.want)
		}
	}
}

func TestEntryAgeClampsNegative(t *testing.T) {
	e := fakeEntry{created: time.Now().Add(30 * time.Second)} // server ahead
	if got := entryAge(e, time.Now()); got != 0 {
		t.Errorf("entryAge = %v, want 0 (clamped)", got)
	}
}

func TestTryAcquireLockWritesOwner(t *testing.T) {
	// Bare-struct pattern (stale_cleanup_test.go:105-111): KV-level, no
	// placement actor needed.
	il := buildBareIdentityLookup(t)
	_, _, ok := il.tryAcquireLock(context.Background(), testCI("k", "id"))
	if !ok {
		t.Fatal("acquire failed")
	}
	entry, _ := il.identities.Get(context.Background(), kvKey(testCI("k", "id")))
	var rec activationRecord
	_ = json.Unmarshal(entry.Value(), &rec)
	if rec.MemberID != il.memberID {
		t.Errorf("lock MemberID = %q, want %q", rec.MemberID, il.memberID)
	}
}
