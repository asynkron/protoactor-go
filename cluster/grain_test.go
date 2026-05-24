package cluster

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestNewGrainCallOptions_RetryActionUsesMilliseconds verifies that the default
// RetryAction sleeps on a millisecond scale, not a nanosecond scale. The first
// retry intends ~50ms (1*1*50). The buggy version used time.Duration(i*i*50)
// without *time.Millisecond, sleeping only ~50ns, which would fail the >=40ms
// lower bound below. The returned counter must still increment (0 -> 1).
func TestNewGrainCallOptions_RetryActionUsesMilliseconds(t *testing.T) {
	c := newClusterForTest("retry-action-test", newInmemoryProvider())
	opts := NewGrainCallOptions(c)

	start := time.Now()
	got := opts.RetryAction(0)
	elapsed := time.Since(start)

	// Return value must remain the incremented loop counter, unchanged by the fix.
	assert.Equal(t, 1, got)

	// Lower bound far above any nanosecond-scale sleep but safely below the
	// intended ~50ms to avoid flakiness from scheduler jitter.
	assert.GreaterOrEqual(t, elapsed, 40*time.Millisecond,
		"first RetryAction should sleep on a millisecond scale (~50ms), got %s", elapsed)
}

func TestWithHeaders_SetsHeadersOnConfig(t *testing.T) {
	t.Parallel()

	cfg := &GrainCallConfig{}
	WithHeaders(map[string]string{"trace": "abc", "tenant": "acme"})(cfg)

	assert.Len(t, cfg.Headers, 2)
	assert.Equal(t, "abc", cfg.Headers["trace"])
	assert.Equal(t, "acme", cfg.Headers["tenant"])
}

func TestWithHeaders_NilMap(t *testing.T) {
	t.Parallel()

	cfg := &GrainCallConfig{}
	WithHeaders(nil)(cfg)
	assert.Nil(t, cfg.Headers)
}
