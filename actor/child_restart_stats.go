package actor

import "time"

type ChildRestartStats struct {
	FailureCount    int
	LastFailureTime time.Time
}

func (c *ChildRestartStats) requestRestartPermission(maxNrOfRetries int, withinTimeMilliseconds int) bool {

	//supervisor says this child may not restart
	if maxNrOfRetries == 0 {
		return false
	}

	//supervisor says child may restart, and we don't care about any timewindow
	if withinTimeMilliseconds == 0 {
		//have we restarted fewer times than supervisor allows?
		return c.FailureCount <= maxNrOfRetries
	}

	return c.FailureCount <= maxNrOfRetries
	//TODO: implement timewindow logic

	return true
}
