package actor

import "time"

type RestartStatistics struct {
	FailureCount    int
	LastFailureTime time.Time
}

func (rs *RestartStatistics) Fail() {
	rs.FailureCount++
}

func (rs *RestartStatistics) Reset() {
	rs.FailureCount = 0
}

func (rs *RestartStatistics) Restart() {
	rs.LastFailureTime = time.Now()
}

func (rs *RestartStatistics) IsWithinDuration(withinDuration time.Duration) bool {
	return time.Since(rs.LastFailureTime) < withinDuration
}
