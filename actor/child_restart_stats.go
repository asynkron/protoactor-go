package actor

import "time"

type RestartStatistics struct {
	FailureCount    int
	LastFailureTime time.Time
}
