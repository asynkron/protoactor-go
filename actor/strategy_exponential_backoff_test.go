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
		{n: "failure outside window; zero count", ft: 11 * time.Second, fc: 10, expected: 0},
		{n: "failure inside window; increment count", ft: 9 * time.Second, fc: 10, expected: 11},
	}

	for _, tc := range cases {
		t.Run(tc.n, func(t *testing.T) {
			s := &exponentialBackoffStrategy{backoffWindow: 10 * time.Second}
			rs := &RestartStatistics{FailureCount: 10, LastFailureTime: time.Now().Add(-tc.ft)}

			s.setFailureCount(rs)
			assert.Equal(t, tc.expected, rs.FailureCount)
		})
	}

}
