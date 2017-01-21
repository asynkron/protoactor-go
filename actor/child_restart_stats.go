package actor

import "time"

type ChildRestartStats struct {
	FailureCount    int
	LastFailureTime time.Time
}

func (c *ChildRestartStats) requestRestartPermission(maxNrOfRetries int, withinDuration time.Duration) bool {

	//supervisor says this child may not restart
	if maxNrOfRetries == 0 {
		return false
	}

	c.FailureCount++

	//supervisor says child may restart, and we don't care about any timewindow
	if withinDuration == 0 {
		//have we restarted fewer times than supervisor allows?
		return c.FailureCount <= maxNrOfRetries
	}

	max := time.Now().Add(-withinDuration)
	if c.LastFailureTime.After(max) {
		return c.FailureCount <= maxNrOfRetries
	}

	//we are past the time limit, we can safely reset the failure count and restart
	c.FailureCount = 0
	return true
}
