package natskv

import (
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/require"
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
