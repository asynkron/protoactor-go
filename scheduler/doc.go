// Package scheduler provides time-based message delivery for actors.
//
// The scheduler package enables sending messages to actors at a future time or at
// regular intervals. It supports one-shot delayed messages, recurring messages,
// and cancellation of scheduled operations.
//
// Key types:
//   - TimerScheduler: Scheduler for sending time-based messages to actors
//   - CancelFunc: Function type for cancelling scheduled operations
//
// Basic usage:
//
//	// Create a scheduler
//	scheduler := scheduler.NewTimerScheduler(system.Root)
//
//	// Send a message after 5 seconds
//	cancel := scheduler.SendOnce(5*time.Second, pid, &DelayedMessage{})
//
//	// Send a message every 10 seconds
//	cancel := scheduler.SendRepeatedly(10*time.Second, 10*time.Second, pid, &RecurringMessage{})
//
//	// Cancel the scheduled operation
//	cancel()
//
//	// Request a message after a delay
//	cancel := scheduler.RequestOnce(2*time.Second, pid, &Query{})
package scheduler
