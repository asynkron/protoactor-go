package actor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestExponentialBackoffStrategy_setFailureCount(t *testing.T) {
	cases := []struct {
		n        string
		ft       time.Duration
		fc       int
		expected int
	}{
		{n: "failure outside window; increment count", ft: 11 * time.Second, fc: 10, expected: 1},
		{n: "failure inside window; increment count", ft: 9 * time.Second, fc: 10, expected: 11},
	}

	for _, tc := range cases {
		t.Run(tc.n, func(t *testing.T) {
			s := &exponentialBackoffStrategy{backoffWindow: 10 * time.Second}
			rs := &RestartStatistics{[]time.Time{}}
			for i := 0; i < tc.fc; i++ {
				rs.failureTimes = append(rs.failureTimes, time.Now().Add(-tc.ft))
			}

			s.setFailureCount(rs)
			assert.Equal(t, tc.expected, rs.FailureCount())
		})
	}
}

func TestExponentialBackoffStrategy_IncrementsFailureCount(t *testing.T) {
	rs := NewRestartStatistics()
	s := &exponentialBackoffStrategy{backoffWindow: 10 * time.Second}

	s.setFailureCount(rs)
	s.setFailureCount(rs)
	s.setFailureCount(rs)

	assert.Equal(t, 3, rs.FailureCount())
}

func TestExponentialBackoffStrategy_ResetsFailureCount(t *testing.T) {
	rs := NewRestartStatistics()
	for i := 0; i < 10; i++ {
		rs.failureTimes = append(rs.failureTimes, time.Now().Add(-11*time.Second))
	}
	s := &exponentialBackoffStrategy{backoffWindow: 10 * time.Second, initialBackoff: 1 * time.Second}

	s.setFailureCount(rs)

	assert.Equal(t, 1, rs.FailureCount())
}
