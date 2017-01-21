package actor

import "time"

type RestartStatistics struct {
	FailureCount    int
	LastFailureTime time.Time
}

func (rs *RestartStatistics) requestRestartPermission(maxNrOfRetries int, withinDuration time.Duration) bool {

	//supervisor says this child may not restart
	if maxNrOfRetries == 0 {
		return false
	}

	rs.FailureCount++

	//supervisor says child may restart, and we don't care about any timewindow
	if withinDuration == 0 {
		//have we restarted fewer times than supervisor allows?
		return rs.FailureCount <= maxNrOfRetries
	}

	max := time.Now().Add(-withinDuration)
	if rs.LastFailureTime.After(max) {
		return rs.FailureCount <= maxNrOfRetries
	}

	//we are past the time limit, we can safely reset the failure count and restart
	rs.FailureCount = 0
	return true
}
