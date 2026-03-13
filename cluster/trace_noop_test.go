package cluster

import (
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/remote"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestNoOpTracer_ClusterOperations_NoPanic(t *testing.T) {
	// Use empty TracerProvider (no exporter attached)
	prevTP := otel.GetTracerProvider()
	otel.SetTracerProvider(sdktrace.NewTracerProvider())
	defer otel.SetTracerProvider(prevTP)

	// Start a minimal single-node cluster using the in-memory test provider
	system := actor.NewActorSystem()
	provider := newInmemoryProvider()
	lookup := &fakeIdentityLookup{}
	remoteConfig := remote.Configure("127.0.0.1", 0)
	clusterConfig := Configure("noop-test-cluster", provider, lookup, remoteConfig)
	c := New(system, clusterConfig)

	c.MemberList = NewMemberList(c)
	c.Remote = remote.NewRemote(system, c.Config.RemoteConfig)
	c.IdentityLookup = lookup

	err := c.StartMember()
	require.NoError(t, err)
	defer c.Shutdown(true)

	// Wait for cluster to initialize
	time.Sleep(2 * time.Second)

	// Make a cluster request — exercises the traced code paths with no-op tracer.
	// This will likely fail (no kind registered) but should NOT panic.
	_, _ = c.Request("test-identity", "nonexistent-kind", &struct{}{})

	// If we reach here without panicking, the test passes
}
