package router

import (
	"testing"

	"github.com/asynkron/protoactor-go/actor"
)

// removeAll removes every routee and waits for the router actor to apply it.
func removeAll(t *testing.T, routerPID *actor.PID, pids ...*actor.PID) {
	t.Helper()

	for _, pid := range pids {
		system.Root.Send(routerPID, &RemoveRoutee{PID: pid})
	}

	res, err := system.Root.RequestFuture(routerPID, &GetRoutees{}, testTimeout).Result()
	if err != nil {
		t.Fatalf("waiting for RemoveRoutee failed: %v", err)
	}
	if n := len(res.(*Routees).PIDs); n != 0 {
		t.Fatalf("expected 0 routees after removal, got %d", n)
	}
}

func TestRoundRobinGroupRouter_SendWithNoRouteesDoesNotPanic(t *testing.T) {
	r0 := spawnRoutee(0)
	defer system.Root.Stop(r0)

	routerPID := system.Root.Spawn(NewRoundRobinGroup(r0))
	defer system.Root.Stop(routerPID)

	if got := request(t, routerPID, 0); got != 0 {
		t.Fatalf("routee did not answer before removal, got %d", got)
	}

	removeAll(t, routerPID, r0)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Send to round robin router with no routees panicked: %v", r)
		}
	}()
	system.Root.Send(routerPID, 1)
}

func TestRandomGroupRouter_SendWithNoRouteesDoesNotPanic(t *testing.T) {
	r0 := spawnRoutee(0)
	defer system.Root.Stop(r0)

	routerPID := system.Root.Spawn(NewRandomGroup(r0))
	defer system.Root.Stop(routerPID)

	if got := request(t, routerPID, 0); got != 0 {
		t.Fatalf("routee did not answer before removal, got %d", got)
	}

	removeAll(t, routerPID, r0)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Send to random router with no routees panicked: %v", r)
		}
	}()
	system.Root.Send(routerPID, 1)
}
