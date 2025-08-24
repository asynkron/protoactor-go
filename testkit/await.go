package testkit

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var pollInterval = 20 * time.Millisecond

// AwaitCondition repeatedly evaluates condition until it returns true or the timeout elapses.
// If the timeout is reached, an error is returned. An optional message overrides the default error text.
func AwaitCondition(ctx context.Context, condition func() bool, timeout time.Duration, message ...string) error {
	if timeout <= 0 {
		timeout = time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		if condition() {
			return nil
		}
		select {
		case <-ctx.Done():
			if len(message) > 0 {
				return errors.New(message[0])
			}
			return fmt.Errorf("condition was not met within %v", timeout)
		case <-time.After(pollInterval):
		}
	}
}

// AwaitConditionAsync is a convenience wrapper using a background context.
func AwaitConditionAsync(condition func() bool, timeout time.Duration, message ...string) error {
	return AwaitCondition(context.Background(), condition, timeout, message...)
}
