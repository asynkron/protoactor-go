package cluster

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrLockNotHeld_IsSentinel(t *testing.T) {
	err := fmt.Errorf("persistence failed: %w", ErrLockNotHeld)
	assert.True(t, errors.Is(err, ErrLockNotHeld),
		"ErrLockNotHeld must be usable as a sentinel with errors.Is")
}

func TestErrLockNotHeld_NotMatchOther(t *testing.T) {
	other := errors.New("some other error")
	assert.False(t, errors.Is(other, ErrLockNotHeld))
}
