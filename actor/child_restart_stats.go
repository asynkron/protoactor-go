package actor

import "time"

//RestartStatistics keeps track of how many times an actor have restarted and when
type RestartStatistics struct {
	FailureCount    int
	LastFailureTime time.Time
}

//Fail increases the associated actors failure count
func (rs *RestartStatistics) Fail() {
	rs.FailureCount++
}

//Reset the associated actors failure count
func (rs *RestartStatistics) Reset() {
	rs.FailureCount = 0
}

//Restart sets the last failure timestamp for the associated actor
func (rs *RestartStatistics) Restart() {
	rs.LastFailureTime = time.Now()
}

//IsWithinDuration checks if a given duration is within the timespan from now to the last falure timestamp
func (rs *RestartStatistics) IsWithinDuration(withinDuration time.Duration) bool {
	return time.Since(rs.LastFailureTime) < withinDuration
}
