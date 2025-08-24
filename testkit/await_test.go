package testkit

import (
	"testing"
	"time"
)

func TestAwaitConditionSuccess(t *testing.T) {
	start := time.Now()
	err := AwaitConditionAsync(func() bool {
		return time.Since(start) > 20*time.Millisecond
	}, time.Second)
	if err != nil {
		t.Fatalf("expected no error: %v", err)
	}
}

func TestAwaitConditionTimeout(t *testing.T) {
	err := AwaitConditionAsync(func() bool { return false }, 50*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}
