package metrics_test

import (
	"log/slog"
	"testing"

	"github.com/awevoke/protoactor-go/metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testLogger() *slog.Logger {
	return slog.Default()
}

func TestNewProtoMetrics_ReturnsNonNil(t *testing.T) {
	pm := metrics.NewProtoMetrics(testLogger())
	require.NotNil(t, pm)
}

func TestNewProtoMetrics_InstrumentsNotNil(t *testing.T) {
	pm := metrics.NewProtoMetrics(testLogger())
	instruments := pm.Instruments()
	require.NotNil(t, instruments)
}

func TestNewProtoMetrics_RegistersInternalMetrics(t *testing.T) {
	pm := metrics.NewProtoMetrics(testLogger())

	// The constructor auto-registers InternalActorMetrics
	got := pm.Get(metrics.InternalActorMetrics)
	require.NotNil(t, got)
	assert.Equal(t, pm.Instruments(), got, "Get(InternalActorMetrics) should return the same Instruments()")
}

func TestNewActorMetrics_AllInstrumentsCreated(t *testing.T) {
	am := metrics.NewActorMetrics(testLogger())
	require.NotNil(t, am)

	assert.NotNil(t, am.ActorFailureCount, "ActorFailureCount should be created")
	assert.NotNil(t, am.ActorMailboxLength, "ActorMailboxLength should be created")
	assert.NotNil(t, am.ActorMessageReceiveDuration, "ActorMessageReceiveDuration should be created")
	assert.NotNil(t, am.ActorRestartedCount, "ActorRestartedCount should be created")
	assert.NotNil(t, am.ActorSpawnCount, "ActorSpawnCount should be created")
	assert.NotNil(t, am.ActorStoppedCount, "ActorStoppedCount should be created")
	assert.NotNil(t, am.DeadLetterCount, "DeadLetterCount should be created")
	assert.NotNil(t, am.FuturesStartedCount, "FuturesStartedCount should be created")
	assert.NotNil(t, am.FuturesCompletedCount, "FuturesCompletedCount should be created")
	assert.NotNil(t, am.FuturesTimedOutCount, "FuturesTimedOutCount should be created")
}

func TestProtoMetrics_RegisterAndGet(t *testing.T) {
	pm := metrics.NewProtoMetrics(testLogger())
	custom := metrics.NewActorMetrics(testLogger())

	pm.Register("custom.metrics", custom)

	got := pm.Get("custom.metrics")
	require.NotNil(t, got)
	assert.Equal(t, custom, got)
}

func TestProtoMetrics_Get_UnknownKey_ReturnsNil(t *testing.T) {
	pm := metrics.NewProtoMetrics(testLogger())

	got := pm.Get("nonexistent.key")
	assert.Nil(t, got)
}

func TestProtoMetrics_Register_DuplicateKey_DoesNotOverwrite(t *testing.T) {
	pm := metrics.NewProtoMetrics(testLogger())
	first := metrics.NewActorMetrics(testLogger())
	second := metrics.NewActorMetrics(testLogger())

	pm.Register("duplicate.key", first)
	pm.Register("duplicate.key", second) // should log error and not overwrite

	got := pm.Get("duplicate.key")
	require.NotNil(t, got)
	assert.Equal(t, first, got, "duplicate registration should not overwrite the first")
}
