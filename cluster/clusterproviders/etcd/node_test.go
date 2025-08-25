package etcd

import (
	"context"
	"log/slog"
	"sync"
	"testing"
)

type recordHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *recordHandler) Enabled(ctx context.Context, level slog.Level) bool { return true }

func (h *recordHandler) Handle(ctx context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r.Clone())
	return nil
}

func (h *recordHandler) WithAttrs(attrs []slog.Attr) slog.Handler { return h }
func (h *recordHandler) WithGroup(name string) slog.Handler       { return h }

func TestStrToIntLogsOnError(t *testing.T) {
	h := &recordHandler{}
	logger := slog.New(h)
	orig := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(orig)

	if v := strToInt("bad"); v != 0 {
		t.Fatalf("expected 0, got %d", v)
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.records) == 0 {
		t.Fatalf("expected log entry for invalid input")
	}
}
