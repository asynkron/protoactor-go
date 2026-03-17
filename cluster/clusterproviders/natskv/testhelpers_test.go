package natskv

import (
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/require"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/remote"
)

// startEmbeddedNATS starts an embedded NATS server with JetStream enabled.
func startEmbeddedNATS(t *testing.T) *server.Server {
	t.Helper()

	opts := &server.Options{
		JetStream: true,
		Port:      -1,
		StoreDir:  t.TempDir(),
	}

	srv, err := server.NewServer(opts)
	require.NoError(t, err, "failed to create NATS server")

	srv.Start()
	t.Cleanup(func() {
		srv.Shutdown()
		srv.WaitForShutdown()
	})

	if !srv.ReadyForConnections(5 * time.Second) {
		t.Fatal("NATS server not ready")
	}

	return srv
}

// connectNATS connects to the given embedded NATS server and returns a connection and JetStream handle.
func connectNATS(t *testing.T, srv *server.Server) (*nats.Conn, jetstream.JetStream) {
	t.Helper()

	nc, err := nats.Connect(srv.ClientURL())
	require.NoError(t, err, "failed to connect to NATS")
	t.Cleanup(func() { nc.Close() })

	js, err := jetstream.New(nc)
	require.NoError(t, err, "failed to create JetStream context")

	return nc, js
}

// setupClusterWithKindsEmbedded creates a provider, actor system, and cluster
// with registered kinds for testing using the embedded NATS server.
func setupClusterWithKindsEmbedded(t *testing.T, srv *server.Server, clusterName string, kinds []*cluster.Kind, opts ...Option) (*Provider, *cluster.Cluster) {
	t.Helper()

	nc, _ := connectNATS(t, srv)

	p, err := New(nc, opts...)
	require.NoError(t, err)

	system := actor.NewActorSystem()
	remoteConfig := remote.Configure("127.0.0.1", 0)
	clusterConfig := cluster.Configure(clusterName, p, p.IdentityLookup(), remoteConfig,
		cluster.WithKinds(kinds...),
	)
	c := cluster.NewCluster(system, clusterConfig)
	c.Remote = remote.NewRemote(system, remoteConfig)

	return p, c
}
