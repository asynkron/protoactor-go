package remote

import (
	"sort"
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
)

// Test registration of multiple kinds via remote config and ensure they can be retrieved.
func TestKindsRegistrationViaConfig(t *testing.T) {
	system := actor.NewActorSystem()
	// Register two dummy actor kinds using WithKinds option.
	config := Configure("localhost", 0,
		WithKinds(
			NewKind("alpha", actor.PropsFromFunc(func(actor.Context) {})),
			NewKind("beta", actor.PropsFromFunc(func(actor.Context) {})),
		),
	)
	remote := NewRemote(system, config)

	kinds := remote.GetKnownKinds()
	sort.Strings(kinds) // ensure deterministic ordering
	assert.Equal(t, []string{"alpha", "beta"}, kinds)
}

// Test requesting an unregistered kind results in an error response.
func TestUnknownKindReturnsError(t *testing.T) {
	system := actor.NewActorSystem()
	remote := NewRemote(system, Configure("localhost", 0))
	remote.Start()
	defer remote.Shutdown(true)

	// Attempt to spawn a kind that has not been registered.
	resp, err := remote.Spawn(system.Address(), "missing", time.Second)
	assert.NoError(t, err)
	assert.Equal(t, ResponseStatusCodeERROR.ToInt32(), resp.StatusCode)
	assert.Nil(t, resp.Pid)
}
