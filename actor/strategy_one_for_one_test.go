package actor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestOneForOneStrategy_requestRestartPermission(t *testing.T) {
	cases := []struct {
		n              string
		expectedResult bool
		expectedCount  int
		s              oneForOneStrategy
		rs             RestartStatistics
	}{
		{
			n: "no restart if max retries is 0",

			s:  oneForOneStrategy{maxNrOfRetries: 0},
			rs: RestartStatistics{},

			expectedResult: true,
			expectedCount:  0,
		},
		{
			n: "restart when duration is 0",

			s:  oneForOneStrategy{maxNrOfRetries: 1},
			rs: RestartStatistics{},

			expectedResult: false,
			expectedCount:  1,
		},
		{
			n: "no restart when duration is 0 and exceeds max retries",

			s:  oneForOneStrategy{maxNrOfRetries: 1},
			rs: RestartStatistics{failureTimes: []time.Time{time.Now().Add(-1 * time.Second)}},

			expectedResult: true,
			expectedCount:  0,
		},
		{
			n: "restart when duration set and within window",

			s:  oneForOneStrategy{maxNrOfRetries: 2, withinDuration: 10 * time.Second},
			rs: RestartStatistics{failureTimes: []time.Time{time.Now().Add(-5 * time.Second)}},

			expectedResult: false,
			expectedCount:  2,
		},
		{
			n: "no restart when duration set, within window and exceeds max retries",

			s:  oneForOneStrategy{maxNrOfRetries: 1, withinDuration: 10 * time.Second},
			rs: RestartStatistics{failureTimes: []time.Time{time.Now().Add(-5 * time.Second), time.Now().Add(-5 * time.Second)}},

			expectedResult: true,
			expectedCount:  0,
		},
		{
			n: "restart and FailureCount reset when duration set and outside window",

			s:  oneForOneStrategy{maxNrOfRetries: 1, withinDuration: 10 * time.Second},
			rs: RestartStatistics{failureTimes: []time.Time{time.Now().Add(-11 * time.Second), time.Now().Add(-11 * time.Second)}},

			expectedResult: false,
			expectedCount:  1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.n, func(t *testing.T) {
			s := tc.s
			rs := tc.rs
			actual := s.shouldStop(&rs)
			assert.Equal(t, tc.expectedResult, actual)
			assert.Equal(t, tc.expectedCount, rs.NumberOfFailures(s.withinDuration))
		})
	}
}
