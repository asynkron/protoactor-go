// Package testkit provides testing utilities for actor systems.
//
// The testkit package offers tools for writing reliable actor tests, including test probes
// for capturing and asserting on messages, await utilities for asynchronous conditions,
// and helpers for cluster testing scenarios.
//
// Key types:
//   - TestProbe: An actor that captures incoming messages for assertion in tests
//   - TestMailboxStatistics: Mailbox statistics collector for testing mailbox behavior
//
// Key functions:
//   - AwaitCondition: Polls a condition until it becomes true or times out
//   - AwaitConditionAsync: Convenience wrapper using a background context
//   - NewInMemClusterFixture: Creates an in-memory cluster for integration testing
//
// Basic usage:
//
//	// Create a test probe
//	probe := testkit.NewTestProbe()
//	system.Root.Spawn(actor.PropsFromFunc(probe.Receive))
//
//	// Send a message and assert on it
//	system.Root.Send(probe.Context().Self(), "test message")
//	msg, err := probe.GetNextMessage(1 * time.Second)
//	assert.NoError(t, err)
//	assert.Equal(t, "test message", msg)
//
//	// Wait for a condition
//	err := testkit.AwaitConditionAsync(func() bool {
//	    return someState == expectedState
//	}, 5*time.Second)
package testkit
