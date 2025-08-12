package remotemetrics

import (
	"io"
	"log/slog"
	"testing"
)

func TestNewRemoteMetrics(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	m := NewRemoteMetrics(logger)
	if m == nil {
		t.Fatalf("expected metrics instance")
	}
	if m.RemoteWriteDuration == nil || m.RemoteActorSpawnCount == nil || m.RemoteSerializedMessageCount == nil ||
		m.RemoteDeserializedMessageCount == nil || m.RemoteEndpointConnectedCount == nil || m.RemoteEndpointDisconnectedCount == nil {
		t.Fatalf("expected all metric instruments to be initialized")
	}
}
