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
}
