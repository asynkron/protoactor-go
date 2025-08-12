package clustermetrics

import (
	"io"
	"log/slog"
	"testing"
)

func TestNewClusterMetrics(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	m := NewClusterMetrics(logger)
	if m == nil {
		t.Fatalf("expected metrics instance")
	}
	if m.VirtualActorsCount == nil || m.ClusterMembersCount == nil {
		t.Fatalf("expected gauges to be initialized")
	}
}
