package remote

import (
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
)

func TestEndpointSupervisor_RestartsChildOnFailure(t *testing.T) {
	config := Configure("127.0.0.1", 0,
		WithSupervisorMaxRestarts(3),
		WithSupervisorRestartWindow(10*time.Second),
	)
	assert.Equal(t, 3, config.SupervisorMaxRestarts)
	assert.Equal(t, 10*time.Second, config.SupervisorRestartWindow)
}

func TestEndpointSupervisor_HandleFailure_RestartsUnderLimit(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("127.0.0.1", 0,
		WithSupervisorMaxRestarts(3),
		WithSupervisorRestartWindow(10*time.Second),
	)
	r := NewRemote(system, config)

	sup := newEndpointSupervisor(r).(*endpointSupervisor)

	// Record a child address mapping (simulating what Receive does)
	childPID := actor.NewPID("127.0.0.1:0", "test-writer")
	sup.childAddresses[childPID.Id] = "10.0.0.1:8080"

	rs := actor.NewRestartStatistics()
	// Simulate 2 failures (under limit of 3)
	rs.Fail()
	rs.Fail()

	mockSupervisor := &mockSupervisorForTest{}
	sup.HandleFailure(system, mockSupervisor, childPID, rs, "test error", nil)

	assert.True(t, mockSupervisor.restartCalled, "should restart child under limit")
	assert.False(t, mockSupervisor.stopCalled, "should not stop child under limit")
}

func TestEndpointSupervisor_HandleFailure_RestartsAtExactLimit(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("127.0.0.1", 0,
		WithSupervisorMaxRestarts(3),
		WithSupervisorRestartWindow(10*time.Second),
	)
	r := NewRemote(system, config)

	sup := newEndpointSupervisor(r).(*endpointSupervisor)

	childPID := actor.NewPID("127.0.0.1:0", "test-writer")
	sup.childAddresses[childPID.Id] = "10.0.0.1:8080"

	rs := actor.NewRestartStatistics()
	// Simulate exactly 3 failures (at limit of 3 — should still restart)
	rs.Fail()
	rs.Fail()
	rs.Fail()

	mockSupervisor := &mockSupervisorForTest{}
	sup.HandleFailure(system, mockSupervisor, childPID, rs, "test error", nil)

	assert.True(t, mockSupervisor.restartCalled, "should restart at exact limit (> not >=)")
	assert.False(t, mockSupervisor.stopCalled, "should not stop at exact limit")
}

func TestEndpointSupervisor_HandleFailure_StopsOverLimit(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("127.0.0.1", 0,
		WithSupervisorMaxRestarts(3),
		WithSupervisorRestartWindow(10*time.Second),
	)
	r := NewRemote(system, config)

	terminated := make(chan string, 1)
	system.EventStream.Subscribe(func(evt any) {
		if e, ok := evt.(*EndpointTerminatedEvent); ok {
			terminated <- e.Address
		}
	})

	sup := newEndpointSupervisor(r).(*endpointSupervisor)

	childPID := actor.NewPID("127.0.0.1:0", "test-writer")
	sup.childAddresses[childPID.Id] = "10.0.0.1:8080"

	rs := actor.NewRestartStatistics()
	// Simulate 4 failures (over limit of 3)
	rs.Fail()
	rs.Fail()
	rs.Fail()
	rs.Fail()

	mockSupervisor := &mockSupervisorForTest{}
	sup.HandleFailure(system, mockSupervisor, childPID, rs, "test error", nil)

	assert.False(t, mockSupervisor.restartCalled, "should not restart child over limit")
	assert.True(t, mockSupervisor.stopCalled, "should stop child over limit")

	select {
	case addr := <-terminated:
		assert.Equal(t, "10.0.0.1:8080", addr)
	case <-time.After(1 * time.Second):
		t.Fatal("expected EndpointTerminatedEvent")
	}
}

// mockSupervisorForTest is a minimal mock for testing HandleFailure logic.
type mockSupervisorForTest struct {
	restartCalled bool
	stopCalled    bool
}

func (m *mockSupervisorForTest) Children() []*actor.PID                    { return nil }
func (m *mockSupervisorForTest) EscalateFailure(reason any, message any)   {}
func (m *mockSupervisorForTest) RestartChildren(pids ...*actor.PID)        { m.restartCalled = true }
func (m *mockSupervisorForTest) StopChildren(pids ...*actor.PID)           { m.stopCalled = true }
func (m *mockSupervisorForTest) ResumeChildren(pids ...*actor.PID)         {}
