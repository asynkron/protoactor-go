package actor

import (
	"time"
)

// RestartStatistics keeps track of how many times an actor have restarted and when
type RestartStatistics struct {
	failureTimes []time.Time
}

// NewRestartStatistics construct a RestartStatistics
func NewRestartStatistics() *RestartStatistics {
	return &RestartStatistics{[]time.Time{}}
}

// FailureCount returns failure count
func (rs *RestartStatistics) FailureCount() int {
	return len(rs.failureTimes)
}

// Fail increases the associated actors failure count
func (rs *RestartStatistics) Fail() {
	rs.failureTimes = append(rs.failureTimes, time.Now())
}

// Reset the associated actors failure count
func (rs *RestartStatistics) Reset() {
	rs.failureTimes = []time.Time{}
}

// NumberOfFailures returns number of failures within a given duration
func (rs *RestartStatistics) NumberOfFailures(withinDuration time.Duration) int {
	if withinDuration == 0 {
		return len(rs.failureTimes)
	}

	num := 0
	currTime := time.Now()
	for _, t := range rs.failureTimes {
		if currTime.Sub(t) < withinDuration {
			num++
		}
	}
	return num
}
