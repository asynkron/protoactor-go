package actor

import "time"

type ChildRestartStats struct {
	FailureCount    int
	LastFailureTime time.Time
}

func inTimeSpan(start, end, check time.Time) bool {
	return check.After(start) && check.Before(end)
}

func (c *ChildRestartStats) requestRestartPermission(maxNrOfRetries int, withinDuration time.Duration) bool {

	//supervisor says this child may not restart
	if maxNrOfRetries == 0 {
		return false
	}

	//supervisor says child may restart, and we don't care about any timewindow
	if withinDuration == 0 {
		//have we restarted fewer times than supervisor allows?
		return c.FailureCount <= maxNrOfRetries
	}

	max := time.Now().Add(-withinDuration)
	if c.LastFailureTime.After(max) {
		return c.FailureCount <= maxNrOfRetries
	}

	//the last event was so long ago that it doesnt matter, lets just say OK to restart
	return true
}
