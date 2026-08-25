package natskv

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The provider's member and leader buckets pass their key TTL straight through
// as LimitMarkerTTL, which the server maps to SubjectDeleteMarkerTTL -- and
// that field has a one-second floor (nats-server server/stream.go:1773-1776,
// "subject delete marker TTL must be at least 1 second"), returning
// JSStreamInvalidConfig. A sub-second MemberTTL or LeaderTTL is therefore a
// startup outage today: createMemberBucket fails, StartMember returns the
// error, and the process never joins the cluster. It is the same defect class
// minTombstoneTTL already fixes for the identity buckets, so it gets the same
// clamp.

func TestClampMarkerTTL(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   time.Duration
		want time.Duration
	}{
		{"zero means no marker TTL", 0, 0},
		{"negative is not passed to the server", -time.Second, 0},
		{"a sub-millisecond value is raised to the floor", time.Nanosecond, minTombstoneTTL},
		{"just under the floor is raised to it", 999 * time.Millisecond, minTombstoneTTL},
		{"the floor itself passes through", minTombstoneTTL, minTombstoneTTL},
		{"a normal value passes through", 5 * time.Second, 5 * time.Second},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, clampMarkerTTL(tc.in))
		})
	}
}

// TestProviderBuckets_SubSecondTTLDoesNotFailStartup is the end-to-end row: a
// 500ms member/leader TTL must still create both buckets, with the KEY TTL left
// exactly as configured (MaxAge has no such floor) and only the MARKER TTL
// raised.
func TestProviderBuckets_SubSecondTTLDoesNotFailStartup(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		ttl           time.Duration
		wantMarkerTTL time.Duration
	}{
		{"below the floor is clamped", 500 * time.Millisecond, minTombstoneTTL},
		{"the floor itself passes through", minTombstoneTTL, minTombstoneTTL},
		{"the default passes through", defaultMemberTTL, defaultMemberTTL},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			srv := startEmbeddedNATS(t)
			nc, _ := connectNATS(t, srv)

			p, err := New(nc, WithMemberTTL(tc.ttl), WithLeaderTTL(tc.ttl))
			require.NoError(t, err)

			p.clusterName = "clamp"
			p.ctx = ctx

			require.NoError(t, p.createMemberBucket(),
				"a sub-second member TTL must not fail provider startup")
			require.NoError(t, p.createLeaderBucket(),
				"a sub-second leader TTL must not fail provider startup")

			memberStatus, err := p.memberBucket.Status(ctx)
			require.NoError(t, err)
			assert.Equal(t, tc.wantMarkerTTL, memberStatus.LimitMarkerTTL(),
				"the member bucket's marker TTL must respect the server floor")
			assert.Equal(t, tc.ttl, memberStatus.TTL(),
				"the KEY TTL is not clamped -- only the marker TTL has a floor")

			leaderStatus, err := p.leaderBucket.Status(ctx)
			require.NoError(t, err)
			assert.Equal(t, tc.wantMarkerTTL, leaderStatus.LimitMarkerTTL())
			assert.Equal(t, tc.ttl, leaderStatus.TTL())
		})
	}
}
