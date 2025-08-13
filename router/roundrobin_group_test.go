package router

import (
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
)

// testTimeout is generously long to reduce flakiness on busy CI systems.
const testTimeout = 5 * time.Second

// spawnRoutee creates an actor that responds with its id for each int message.
func spawnRoutee(id int) *actor.PID {
	props := actor.PropsFromFunc(func(ctx actor.Context) {
		if _, ok := ctx.Message().(int); ok {
			ctx.Respond(id)
		}
	})
	return system.Root.Spawn(props)
}

// request sends msg to pid and waits for an int response.
func request(t *testing.T, pid *actor.PID, msg interface{}) int {
	f := system.Root.RequestFuture(pid, msg, testTimeout)
	res, err := f.Result()
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	return res.(int)
}

// assertCycle verifies that got contains complete round-robin cycles of size n.
func assertCycle(t *testing.T, got []int, n int) {
	if len(got)%n != 0 {
		t.Fatalf("got %d messages, not a multiple of %d", len(got), n)
	}
	cycle := got[:n]
	seen := make(map[int]struct{}, n)
	for _, id := range cycle {
		if _, ok := seen[id]; ok {
			t.Fatalf("routee %d received multiple messages in first cycle %v", id, cycle)
		}
		seen[id] = struct{}{}
	}
	if len(seen) != n {
		t.Fatalf("first cycle %v does not contain %d unique routees", cycle, n)
	}
	for i := n; i < len(got); i++ {
		expected := cycle[i%n]
		if got[i] != expected {
			t.Fatalf("expected %v at position %d but got %v (cycle %v)", expected, i, got[i], cycle)
		}
	}
}

func TestRoundRobinGroupRouter_RoutesMessagesInRoundRobinOrder(t *testing.T) {
	var routees []*actor.PID
	for i := 0; i < 3; i++ {
		pid := spawnRoutee(i)
		defer system.Root.Stop(pid)
		routees = append(routees, pid)
	}
	routerPID := system.Root.Spawn(NewRoundRobinGroup(routees...))
	defer system.Root.Stop(routerPID)

	msgCount := len(routees) * 2
	results := make([]int, msgCount)
	for i := 0; i < msgCount; i++ {
		results[i] = request(t, routerPID, i)
	}

	assertCycle(t, results, len(routees))
}

func TestRoundRobinGroupRouter_AddAndRemoveRoutees(t *testing.T) {
	r0 := spawnRoutee(0)
	r1 := spawnRoutee(1)
	defer system.Root.Stop(r0)
	defer system.Root.Stop(r1)

	routerPID := system.Root.Spawn(NewRoundRobinGroup(r0, r1))
	defer system.Root.Stop(routerPID)

	// initial round-robin with two routees
	initial := make([]int, 4)
	for i := 0; i < 4; i++ {
		initial[i] = request(t, routerPID, i)
	}
	assertCycle(t, initial, 2)

	// add a third routee
	r2 := spawnRoutee(2)
	defer system.Root.Stop(r2)
	system.Root.Send(routerPID, &AddRoutee{PID: r2})
	if _, err := system.Root.RequestFuture(routerPID, &GetRoutees{}, testTimeout).Result(); err != nil {
		t.Fatalf("waiting for AddRoutee failed: %v", err)
	}

	afterAdd := make([]int, 6)
	for i := 0; i < 6; i++ {
		afterAdd[i] = request(t, routerPID, i)
	}
	assertCycle(t, afterAdd, 3)

	// ensure new routee participates
	found := false
	for _, id := range afterAdd[:3] {
		if id == 2 {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("added routee did not receive any messages: %v", afterAdd[:3])
	}

	// remove one routee
	system.Root.Send(routerPID, &RemoveRoutee{PID: r1})
	if _, err := system.Root.RequestFuture(routerPID, &GetRoutees{}, testTimeout).Result(); err != nil {
		t.Fatalf("waiting for RemoveRoutee failed: %v", err)
	}

	afterRemove := make([]int, 4)
	for i := 0; i < 4; i++ {
		afterRemove[i] = request(t, routerPID, i)
	}
	for _, id := range afterRemove {
		if id == 1 {
			t.Fatalf("removed routee received message: %v", afterRemove)
		}
	}
	assertCycle(t, afterRemove, 2)
}
