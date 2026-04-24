# Ring 1: Core Safety Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Eliminate all code paths that crash the process or silently corrupt state in protoactor-go.

**Architecture:** Fix 11 issues across remote, cluster, and actor packages. Each fix follows TDD: write failing test, implement minimal fix, verify, commit. Ordered by complexity (hardest first).

**Tech Stack:** Go, gRPC, NATS JetStream KV, testify (assert/require), proto3

**Design doc:** `docs/plans/2026-02-23-ring1-core-safety-design.md`

---

### Task 1: Fix mustEmbedUnimplementedRemotingServer panic

**Files:**
- Modify: `remote/endpoint_reader.go:23-26`

**Step 1: Verify the panic exists**

Read `remote/endpoint_reader.go` lines 23-26 and confirm the method body contains `panic("implement me")`.

**Step 2: Write the failing test**

Add to `remote/endpoint_reader_test.go`:

```go
func TestEndpointReader_MustEmbedDoesNotPanic(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("localhost", 0)
	r := NewRemote(system, config)
	reader := newEndpointReader(r)

	assert.NotPanics(t, func() {
		reader.mustEmbedUnimplementedRemotingServer()
	})
}
```

**Step 3: Run test to verify it fails**

Run: `go test ./remote/ -run TestEndpointReader_MustEmbedDoesNotPanic -v`
Expected: FAIL with panic "implement me"

**Step 4: Implement the fix**

In `remote/endpoint_reader.go`, replace the method body:

```go
func (s *endpointReader) mustEmbedUnimplementedRemotingServer() {
	// Required for gRPC forward-compatibility. No-op by design.
}
```

**Step 5: Run test to verify it passes**

Run: `go test ./remote/ -run TestEndpointReader_MustEmbedDoesNotPanic -v`
Expected: PASS

**Step 6: Commit**

```bash
git add remote/endpoint_reader.go remote/endpoint_reader_test.go
git commit -m "fix(remote): remove panic from mustEmbedUnimplementedRemotingServer

This is a gRPC forward-compatibility marker method that must exist
but should be a no-op."
```

---

### Task 2: Implement ListProcesses RPC

**Files:**
- Modify: `remote/endpoint_reader.go:28-30`
- Modify: `remote/endpoint_reader_test.go`

The proto defines `ListProcessesRequest` with:
- `string pattern = 1;` — filter pattern
- `ListProcessesMatchType type = 2;` — enum: `MatchPartOfString` (0), `MatchExactString` (1), `MatchRegex` (2)

`ListProcessesResponse` has `repeated actor.PID pids = 1;`.

The `ProcessRegistryValue.LocalPIDs` is a `*SliceMap` containing `[]cmap.ConcurrentMap` (1024 shards). Each shard has `.Keys()` and `.Get()` methods.

**Step 1: Write the failing test**

Add to `remote/endpoint_reader_test.go`:

```go
import (
	"context"
	"regexp"
	"strings"
	// ... existing imports
)

func TestListProcesses_ReturnsSpawnedActors(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("localhost", 0)
	r := NewRemote(system, config)
	reader := newEndpointReader(r)

	// Spawn a test actor so there's at least one process in the registry
	props := actor.PropsFromFunc(func(ctx actor.Context) {})
	pid, err := system.Root.SpawnNamed(props, "test-list-actor")
	require.NoError(t, err)
	defer system.Root.Stop(pid)

	resp, err := reader.ListProcesses(context.Background(), &ListProcessesRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Should contain at least the actor we spawned
	found := false
	for _, p := range resp.Pids {
		if p.Id == "test-list-actor" {
			found = true
			break
		}
	}
	assert.True(t, found, "ListProcesses should return the spawned actor")
}

func TestListProcesses_FilterByPattern(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("localhost", 0)
	r := NewRemote(system, config)
	reader := newEndpointReader(r)

	props := actor.PropsFromFunc(func(ctx actor.Context) {})
	pid1, _ := system.Root.SpawnNamed(props, "alpha-actor")
	pid2, _ := system.Root.SpawnNamed(props, "beta-actor")
	defer system.Root.Stop(pid1)
	defer system.Root.Stop(pid2)

	// MatchPartOfString (default): filter for "alpha"
	resp, err := reader.ListProcesses(context.Background(), &ListProcessesRequest{
		Pattern: "alpha",
		Type:    ListProcessesMatchType_MatchPartOfString,
	})
	require.NoError(t, err)

	foundAlpha := false
	foundBeta := false
	for _, p := range resp.Pids {
		if p.Id == "alpha-actor" {
			foundAlpha = true
		}
		if p.Id == "beta-actor" {
			foundBeta = true
		}
	}
	assert.True(t, foundAlpha, "should find alpha-actor with partial match")
	assert.False(t, foundBeta, "should not find beta-actor with alpha filter")
}

func TestListProcesses_FilterByExactMatch(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("localhost", 0)
	r := NewRemote(system, config)
	reader := newEndpointReader(r)

	props := actor.PropsFromFunc(func(ctx actor.Context) {})
	pid1, _ := system.Root.SpawnNamed(props, "exact-match-actor")
	defer system.Root.Stop(pid1)

	// MatchExactString
	resp, err := reader.ListProcesses(context.Background(), &ListProcessesRequest{
		Pattern: "exact-match-actor",
		Type:    ListProcessesMatchType_MatchExactString,
	})
	require.NoError(t, err)

	found := false
	for _, p := range resp.Pids {
		if p.Id == "exact-match-actor" {
			found = true
		}
	}
	assert.True(t, found, "exact match should find the actor")

	// Partial should not match with exact type
	resp, err = reader.ListProcesses(context.Background(), &ListProcessesRequest{
		Pattern: "exact-match",
		Type:    ListProcessesMatchType_MatchExactString,
	})
	require.NoError(t, err)

	found = false
	for _, p := range resp.Pids {
		if p.Id == "exact-match-actor" {
			found = true
		}
	}
	assert.False(t, found, "partial string should not match with exact match type")
}

func TestListProcesses_FilterByRegex(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("localhost", 0)
	r := NewRemote(system, config)
	reader := newEndpointReader(r)

	props := actor.PropsFromFunc(func(ctx actor.Context) {})
	pid1, _ := system.Root.SpawnNamed(props, "regex-test-123")
	pid2, _ := system.Root.SpawnNamed(props, "regex-test-abc")
	defer system.Root.Stop(pid1)
	defer system.Root.Stop(pid2)

	// MatchRegex: digits at end
	resp, err := reader.ListProcesses(context.Background(), &ListProcessesRequest{
		Pattern: `regex-test-\d+`,
		Type:    ListProcessesMatchType_MatchRegex,
	})
	require.NoError(t, err)

	found123 := false
	foundAbc := false
	for _, p := range resp.Pids {
		if p.Id == "regex-test-123" {
			found123 = true
		}
		if p.Id == "regex-test-abc" {
			foundAbc = true
		}
	}
	assert.True(t, found123, "regex should match digits suffix")
	assert.False(t, foundAbc, "regex should not match alpha suffix")
}

func TestListProcesses_InvalidRegex(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("localhost", 0)
	r := NewRemote(system, config)
	reader := newEndpointReader(r)

	resp, err := reader.ListProcesses(context.Background(), &ListProcessesRequest{
		Pattern: "[invalid",
		Type:    ListProcessesMatchType_MatchRegex,
	})
	assert.Error(t, err, "invalid regex should return error")
	assert.Nil(t, resp)
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./remote/ -run "TestListProcesses" -v`
Expected: FAIL with panic "implement me"

**Step 3: Implement ListProcesses**

In `remote/endpoint_reader.go`, replace the `ListProcesses` method:

```go
func (s *endpointReader) ListProcesses(_ context.Context, request *ListProcessesRequest) (*ListProcessesResponse, error) {
	registry := s.remote.actorSystem.ProcessRegistry

	// Determine the match function based on the request type.
	var matchFn func(id string) bool
	pattern := request.GetPattern()

	if pattern == "" {
		matchFn = func(string) bool { return true }
	} else {
		switch request.GetType() {
		case ListProcessesMatchType_MatchExactString:
			matchFn = func(id string) bool { return id == pattern }
		case ListProcessesMatchType_MatchRegex:
			re, err := regexp.Compile(pattern)
			if err != nil {
				return nil, fmt.Errorf("invalid regex pattern: %w", err)
			}
			matchFn = re.MatchString
		default: // MatchPartOfString
			matchFn = func(id string) bool { return strings.Contains(id, pattern) }
		}
	}

	var pids []*actor.PID
	for _, shard := range registry.LocalPIDs.LocalPIDs {
		for _, id := range shard.Keys() {
			if matchFn(id) {
				pids = append(pids, &actor.PID{
					Address: registry.Address,
					Id:      id,
				})
			}
		}
	}

	return &ListProcessesResponse{Pids: pids}, nil
}
```

Add `"fmt"`, `"regexp"`, `"strings"` to the import block if not already present.

**Step 4: Run tests to verify they pass**

Run: `go test ./remote/ -run "TestListProcesses" -v -race`
Expected: PASS

**Step 5: Commit**

```bash
git add remote/endpoint_reader.go remote/endpoint_reader_test.go
git commit -m "feat(remote): implement ListProcesses RPC

Enumerates processes from the actor system's ProcessRegistry with
support for partial string, exact match, and regex filtering."
```

---

### Task 3: Implement GetProcessDiagnostics RPC

**Files:**
- Modify: `remote/endpoint_reader.go:32-34`
- Modify: `remote/endpoint_reader_test.go`

The proto defines:
- `GetProcessDiagnosticsRequest` with `actor.PID pid = 1;`
- `GetProcessDiagnosticsResponse` with `string diagnostics_string = 1;`

The `ActorSystem` has `Config.DiagnosticsSerializer func(Actor) string`. The `ActorProcess` struct (in `actor/actor_process.go`) has a `mailbox` field but no direct access to the actor. We'll return basic info: whether the process exists and its Go type name.

**Step 1: Write the failing test**

Add to `remote/endpoint_reader_test.go`:

```go
func TestGetProcessDiagnostics_ExistingProcess(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("localhost", 0)
	r := NewRemote(system, config)
	reader := newEndpointReader(r)

	props := actor.PropsFromFunc(func(ctx actor.Context) {})
	pid, err := system.Root.SpawnNamed(props, "diag-actor")
	require.NoError(t, err)
	defer system.Root.Stop(pid)

	resp, err := reader.GetProcessDiagnostics(context.Background(), &GetProcessDiagnosticsRequest{
		Pid: pid,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.NotEmpty(t, resp.DiagnosticsString, "should return diagnostics for existing process")
}

func TestGetProcessDiagnostics_NonExistentProcess(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("localhost", 0)
	r := NewRemote(system, config)
	reader := newEndpointReader(r)

	resp, err := reader.GetProcessDiagnostics(context.Background(), &GetProcessDiagnosticsRequest{
		Pid: actor.NewPID("nonhost", "nonexistent"),
	})
	assert.Error(t, err, "should return error for non-existent process")
	assert.Nil(t, resp)
}

func TestGetProcessDiagnostics_NilPid(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("localhost", 0)
	r := NewRemote(system, config)
	reader := newEndpointReader(r)

	resp, err := reader.GetProcessDiagnostics(context.Background(), &GetProcessDiagnosticsRequest{
		Pid: nil,
	})
	assert.Error(t, err, "should return error for nil pid")
	assert.Nil(t, resp)
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./remote/ -run "TestGetProcessDiagnostics" -v`
Expected: FAIL with panic "implement me"

**Step 3: Implement GetProcessDiagnostics**

In `remote/endpoint_reader.go`, replace the `GetProcessDiagnostics` method:

```go
func (s *endpointReader) GetProcessDiagnostics(_ context.Context, request *GetProcessDiagnosticsRequest) (*GetProcessDiagnosticsResponse, error) {
	pid := request.GetPid()
	if pid == nil {
		return nil, fmt.Errorf("pid is required")
	}

	process, ok := s.remote.actorSystem.ProcessRegistry.GetLocal(pid.Id)
	if !ok {
		return nil, fmt.Errorf("process not found: %s", pid.Id)
	}

	diagnostics := fmt.Sprintf("process_type:%T", process)
	return &GetProcessDiagnosticsResponse{DiagnosticsString: diagnostics}, nil
}
```

Add `"fmt"` to the import block if not already present.

**Step 4: Run tests to verify they pass**

Run: `go test ./remote/ -run "TestGetProcessDiagnostics" -v -race`
Expected: PASS

**Step 5: Commit**

```bash
git add remote/endpoint_reader.go remote/endpoint_reader_test.go
git commit -m "feat(remote): implement GetProcessDiagnostics RPC

Returns process type information for a given PID, or an error
if the process doesn't exist."
```

---

### Task 4: Implement ClientConnection handling

**Files:**
- Modify: `remote/endpoint_reader.go:114-118`
- Modify: `remote/endpoint_reader_test.go`

The `ConnectResponse` proto has: `string member_id = 2;` and `bool blocked = 3;`.

The `ServerConnection` handler (`onServerConnection` at line 235) sends a `ConnectResponse` with the server's member ID and whether the client is blocked. `ClientConnection` should do the same — respond with the server's member ID and blocked status — but does NOT need to register the client in any topology or endpoint tracking.

**Step 1: Write the failing test**

Add to `remote/endpoint_reader_test.go`:

```go
type mockReceiveServer struct {
	Remoting_ReceiveServer
	sentMessages []*RemoteMessage
}

func (m *mockReceiveServer) Send(msg *RemoteMessage) error {
	m.sentMessages = append(m.sentMessages, msg)
	return nil
}

func TestOnConnectRequest_ClientConnection(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("localhost", 0)
	r := NewRemote(system, config)
	reader := newEndpointReader(r)

	stream := &mockReceiveServer{}
	_, err := reader.OnConnectRequest(stream, &ConnectRequest{
		ConnectionType: &ConnectRequest_ClientConnection{
			ClientConnection: &ClientConnection{
				MemberId: "client-1",
			},
		},
	})
	require.NoError(t, err)

	require.Len(t, stream.sentMessages, 1, "should send a ConnectResponse")
	connectResp := stream.sentMessages[0].GetConnectResponse()
	require.NotNil(t, connectResp, "response should be a ConnectResponse")
	assert.Equal(t, system.ID, connectResp.MemberId)
	assert.False(t, connectResp.Blocked)
}

func TestOnConnectRequest_ClientConnection_Blocked(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("localhost", 0)
	r := NewRemote(system, config)
	reader := newEndpointReader(r)

	// Block the client member
	r.BlockList().Block([]string{"blocked-client"})

	stream := &mockReceiveServer{}
	_, err := reader.OnConnectRequest(stream, &ConnectRequest{
		ConnectionType: &ConnectRequest_ClientConnection{
			ClientConnection: &ClientConnection{
				MemberId: "blocked-client",
			},
		},
	})
	require.NoError(t, err)

	require.Len(t, stream.sentMessages, 1)
	connectResp := stream.sentMessages[0].GetConnectResponse()
	require.NotNil(t, connectResp)
	assert.True(t, connectResp.Blocked)
	assert.Equal(t, system.ID, connectResp.MemberId)
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./remote/ -run "TestOnConnectRequest_ClientConnection" -v`
Expected: FAIL — the current code logs error and sends no response.

**Step 3: Implement ClientConnection handling**

In `remote/endpoint_reader.go`, replace the `ClientConnection` case in `OnConnectRequest`:

```go
case *ConnectRequest_ClientConnection:
	{
		cc := connType.ClientConnection
		s.onClientConnection(stream, cc)
	}
```

Then add a new method after `onServerConnection`:

```go
func (s *endpointReader) onClientConnection(stream Remoting_ReceiveServer, cc *ClientConnection) {
	blocked := s.remote.BlockList().IsBlocked(cc.MemberId)
	err := stream.Send(&RemoteMessage{
		MessageType: &RemoteMessage_ConnectResponse{
			ConnectResponse: &ConnectResponse{
				Blocked:  blocked,
				MemberId: s.remote.actorSystem.ID,
			},
		},
	})
	if err != nil {
		s.remote.Logger().Error("EndpointReader failed to send ConnectResponse for client", slog.Any("error", err))
	}
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./remote/ -run "TestOnConnectRequest_ClientConnection" -v -race`
Expected: PASS

**Step 5: Run all existing endpoint_reader tests to check for regressions**

Run: `go test ./remote/ -v -race`
Expected: All PASS

**Step 6: Commit**

```bash
git add remote/endpoint_reader.go remote/endpoint_reader_test.go
git commit -m "feat(remote): implement ClientConnection handling in endpoint reader

Responds with server member ID and blocked status, matching the
ServerConnection pattern but without registering in cluster topology."
```

---

### Task 5: Eliminate endpoint writer panic

**Files:**
- Modify: `remote/endpoint_writer.go:358-360`
- Modify: `remote/endpoint_writer_test.go`

The current code at line 360: `panic(msg.err)` in the `*restartAfterConnectFailure` case.

**Step 1: Write the failing test**

Add to `remote/endpoint_writer_test.go`:

```go
func TestEndpointWriter_RestartAfterConnectFailure_DoesNotPanic(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("localhost", 0)
	r := NewRemote(system, config)

	writer := &endpointWriter{
		address: "127.0.0.1:12345",
		config:  r.config,
		remote:  r,
	}

	// Subscribe to EndpointTerminatedEvent to confirm it's published
	terminated := make(chan *EndpointTerminatedEvent, 1)
	system.EventStream.Subscribe(func(evt any) {
		if e, ok := evt.(*EndpointTerminatedEvent); ok {
			terminated <- e
		}
	})

	// Create a mock context that tracks Stop calls
	props := actor.PropsFromFunc(func(ctx actor.Context) {
		switch msg := ctx.Message().(type) {
		case *restartAfterConnectFailure:
			// This is the message we're testing
			_ = msg
		}
	})
	pid := system.Root.Spawn(props)
	defer system.Root.Stop(pid)

	// Send the restartAfterConnectFailure message and verify no panic
	assert.NotPanics(t, func() {
		system.Root.Send(pid, &restartAfterConnectFailure{err: fmt.Errorf("connection refused")})
		time.Sleep(100 * time.Millisecond)
	})
}
```

Note: This test is tricky because the `endpointWriter.Receive` method is called by the actor framework. A simpler approach is to test that the code path doesn't panic. However, since the `restartAfterConnectFailure` handler needs access to `state.remote` and `state.address`, a more practical test uses the actual actor:

```go
func TestEndpointWriter_RestartAfterConnectFailure_DoesNotPanic(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("localhost", 0)
	r := NewRemote(system, config)

	// Subscribe to EndpointTerminatedEvent
	terminated := make(chan string, 1)
	system.EventStream.Subscribe(func(evt any) {
		if e, ok := evt.(*EndpointTerminatedEvent); ok {
			terminated <- e.Address
		}
	})

	address := "127.0.0.1:99999"
	props := actor.PropsFromProducer(endpointWriterProducer(r, address, r.config),
		actor.WithMailbox(actor.Unbounded(&actor.DroppedMessageHandler{})),
	)
	pid := system.Root.Spawn(props)

	// The writer will fail to connect during initialization.
	// Verify an EndpointTerminatedEvent is published (not a panic).
	select {
	case addr := <-terminated:
		assert.Equal(t, address, addr)
	case <-time.After(30 * time.Second):
		t.Fatal("expected EndpointTerminatedEvent but timed out")
	}

	system.Root.Stop(pid)
}
```

Actually, the simplest test is to verify the code path directly. Since `restartAfterConnectFailure` is currently only reachable via a dead code path (the `time.AfterFunc` is commented out at line 76), the panic at line 360 is dormant. We should still fix it. The most robust test:

Add to `remote/endpoint_writer_test.go`:

```go
import (
	"fmt"
	"testing"
	"time"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
)

func TestEndpointWriter_RestartAfterConnectFailure_NoPanic(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("localhost", 0)
	r := NewRemote(system, config)

	// Capture EndpointTerminatedEvent
	terminated := make(chan string, 1)
	system.EventStream.Subscribe(func(evt any) {
		if e, ok := evt.(*EndpointTerminatedEvent); ok {
			terminated <- e.Address
		}
	})

	address := "127.0.0.1:99999"
	writer := &endpointWriter{
		address: address,
		config:  config,
		remote:  r,
	}

	// Create a real actor with this writer's Receive method
	props := actor.PropsFromProducer(func() actor.Actor { return writer })
	pid := system.Root.Spawn(props)

	// Send the restartAfterConnectFailure message
	system.Root.Send(pid, &restartAfterConnectFailure{err: fmt.Errorf("connection refused")})

	select {
	case addr := <-terminated:
		assert.Equal(t, address, addr)
	case <-time.After(5 * time.Second):
		t.Fatal("expected EndpointTerminatedEvent, got timeout")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./remote/ -run "TestEndpointWriter_RestartAfterConnectFailure_NoPanic" -v`
Expected: FAIL with panic from `panic(msg.err)`

**Step 3: Implement the fix**

In `remote/endpoint_writer.go`, replace the `*restartAfterConnectFailure` case (lines 358-360):

```go
	case *restartAfterConnectFailure:
		state.remote.Logger().Error("EndpointWriter connect failure, terminating endpoint",
			slog.String("address", state.address), slog.Any("error", msg.err))
		terminated := &EndpointTerminatedEvent{Address: state.address}
		state.remote.actorSystem.EventStream.Publish(terminated)
		ctx.Stop(ctx.Self())
```

**Step 4: Run test to verify it passes**

Run: `go test ./remote/ -run "TestEndpointWriter_RestartAfterConnectFailure_NoPanic" -v -race`
Expected: PASS

**Step 5: Run all remote tests**

Run: `go test ./remote/ -v -race`
Expected: All PASS

**Step 6: Commit**

```bash
git add remote/endpoint_writer.go remote/endpoint_writer_test.go
git commit -m "fix(remote): replace panic with graceful termination in endpoint writer

restartAfterConnectFailure now publishes EndpointTerminatedEvent and
stops the actor instead of panicking."
```

---

### Task 6: Fix consul provider actor panic

**Files:**
- Modify: `cluster/clusterproviders/consul/provider_actor.go:97-101`

**Step 1: Read the current code**

Read `cluster/clusterproviders/consul/provider_actor.go` and confirm the panic at line 100.

**Step 2: Implement the fix**

Replace the goroutine in `startWatch` (lines 97-102):

```go
	go func() {
		if err = plan.RunWithConfig(pa.consulConfig.Address, pa.consulConfig); err != nil {
			ctx.Logger().Error("Failed to start consul watch", slog.Any("error", err))
			ctx.Poison(ctx.Self())
		}
	}()
```

This replaces `panic(err)` with `ctx.Poison(ctx.Self())`, which gracefully stops the actor after processing remaining messages.

**Step 3: Run existing consul tests to verify no regressions**

Run: `go test ./cluster/clusterproviders/consul/ -v -race`
Expected: PASS (existing tests may need build tags; check with `-tags integration` if needed)

If tests require a consul instance, at minimum verify the code compiles:
Run: `go build ./cluster/clusterproviders/consul/`
Expected: Success

**Step 4: Commit**

```bash
git add cluster/clusterproviders/consul/provider_actor.go
git commit -m "fix(consul): replace panic with Poison in provider actor watch

When the consul watch fails, gracefully stop the actor instead of
crashing the process."
```

---

### Task 7: Fix proto.Clone nil safety

**Files:**
- Modify: `remote/endpoint_reader.go:220` (in `deserializeSender`)
- Modify: `remote/endpoint_writer.go:336` (in `addToSenderLookup`)
- Modify: `remote/endpoint_reader_test.go`

**Step 1: Write the failing test**

`deserializeSender` at line 220 calls `proto.Clone(pid)` when `requestID > 0`. If the `pid` from the array is nil, this will panic on `pid.RequestId = requestID` at line 221.

Add to `remote/endpoint_reader_test.go`:

```go
func TestDeserializeSender_NilPidInArray(t *testing.T) {
	// Array has a nil entry
	arr := []*actor.PID{nil}

	// index=1 (1-based) with requestID > 0 triggers the Clone path
	assert.NotPanics(t, func() {
		result := deserializeSender(1, 42, arr)
		assert.Nil(t, result, "nil pid in array should return nil")
	})
}
```

For `addToSenderLookup`, the nil PID case is already handled by the early return at line 328. The `proto.Clone(pid)` at line 336 is only reached when `pid != nil`, so it's safe. No test needed for that path.

**Step 2: Run test to verify it fails**

Run: `go test ./remote/ -run "TestDeserializeSender_NilPidInArray" -v`
Expected: FAIL with nil pointer dereference

**Step 3: Implement the fix**

In `remote/endpoint_reader.go`, in the `deserializeSender` function, add a nil check. The current code (lines 209-224):

```go
func deserializeSender(index int32, requestID uint32, arr []*actor.PID) *actor.PID {
	if index == 0 {
		return nil
	}
	if index < 0 || int(index-1) >= len(arr) {
		return nil
	}
	pid := arr[index-1]

	// if request id is used, clone the PID first so we don't corrupt the lookup
	if requestID > 0 {
		pid, _ = proto.Clone(pid).(*actor.PID)
		pid.RequestId = requestID
	}
	return pid
}
```

Replace with:

```go
func deserializeSender(index int32, requestID uint32, arr []*actor.PID) *actor.PID {
	if index == 0 {
		return nil
	}
	if index < 0 || int(index-1) >= len(arr) {
		return nil
	}
	pid := arr[index-1]
	if pid == nil {
		return nil
	}

	// if request id is used, clone the PID first so we don't corrupt the lookup
	if requestID > 0 {
		pid, _ = proto.Clone(pid).(*actor.PID)
		pid.RequestId = requestID
	}
	return pid
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./remote/ -run "TestDeserializeSender_NilPidInArray" -v -race`
Expected: PASS

**Step 5: Run all remote tests for regressions**

Run: `go test ./remote/ -v -race`
Expected: All PASS

**Step 6: Commit**

```bash
git add remote/endpoint_reader.go remote/endpoint_reader_test.go
git commit -m "fix(remote): prevent nil pointer panic in deserializeSender

Add nil check before proto.Clone to handle nil PIDs in the
sender array."
```

---

### Task 8: Fix NATS identity error suppression

**Files:**
- Modify: `cluster/identitylookup/nats/nats_identity.go` — lines 372, 406, 438, 439

These are in the `NatsIdentityStorage` struct's `addKeyToMember` and `removeKeyFromMember` methods.

**Step 1: Read the code and identify the exact lines**

Read `cluster/identitylookup/nats/nats_identity.go` lines 365-440.

Current suppressed errors:
- Line 372: `data, _ := json.Marshal(&mrec)` in `addKeyToMember` (create path)
- Line 406: `data, _ := json.Marshal(&mrec)` in `addKeyToMember` (update path)
- Line 438: `data, _ := json.Marshal(&mrec)` in `removeKeyFromMember`
- Line 439: `_, _ = s.members.Update(ctx, memberID, data, entry.Revision())` in `removeKeyFromMember`

**Step 2: Implement the fixes**

For line 372 in `addKeyToMember` (inside the `ErrKeyNotFound` branch):

Replace:
```go
data, _ := json.Marshal(&mrec)
```
With:
```go
data, err := json.Marshal(&mrec)
if err != nil {
	slog.Error("NATS identity: addKeyToMember marshal failed",
		slog.String("memberID", memberID), slog.Any("error", err))
	return
}
```

For line 406 in `addKeyToMember` (the update path):

Replace:
```go
data, _ := json.Marshal(&mrec)
```
With:
```go
data, err := json.Marshal(&mrec)
if err != nil {
	slog.Error("NATS identity: addKeyToMember marshal failed",
		slog.String("memberID", memberID), slog.Any("error", err))
	return
}
```

For line 438 in `removeKeyFromMember`:

Replace:
```go
data, _ := json.Marshal(&mrec)
_, _ = s.members.Update(ctx, memberID, data, entry.Revision())
```
With:
```go
data, err := json.Marshal(&mrec)
if err != nil {
	slog.Error("NATS identity: removeKeyFromMember marshal failed",
		slog.String("memberID", memberID), slog.Any("error", err))
	return
}
if _, err := s.members.Update(ctx, memberID, data, entry.Revision()); err != nil {
	slog.Warn("NATS identity: removeKeyFromMember CAS update failed, will be cleaned up on member leave",
		slog.String("memberID", memberID), slog.Any("error", err))
}
```

**Step 3: Verify compilation**

Run: `go build ./cluster/identitylookup/nats/`
Expected: Success

**Step 4: Run existing tests**

Run: `go test ./cluster/identitylookup/nats/ -v -race`
Expected: All PASS

**Step 5: Commit**

```bash
git add cluster/identitylookup/nats/nats_identity.go
git commit -m "fix(nats): handle suppressed errors in identity storage

Log json.Marshal and CAS update errors instead of silently discarding
them in addKeyToMember and removeKeyFromMember."
```

---

### Task 9: Fix Postgres identity error suppression

**Files:**
- Modify: `cluster/identitylookup/postgres/postgres_identity.go` — lines 129, 143-144 (in the `TryAcquireLock` method), and 276

**Step 1: Read the code and identify exact lines**

Read `cluster/identitylookup/postgres/postgres_identity.go`.

Current suppressed errors:
- Line 129: `_, _ = s.db.ExecContext(ctx, cleanQuery, key)` in `TryAcquireLock`
- Lines 143-144: `rows, err := result.RowsAffected()` / `if err != nil || rows == 0` — this already checks err, but the original design doc says line 276 has `rows, _ := result.RowsAffected()`. Let me verify.

Actually looking at the file content:
- Line 129: `_, _ = s.db.ExecContext(ctx, cleanQuery, key)` — suppressed error
- Line 143: `rows, err := result.RowsAffected()` — this already handles err (at line 144: `if err != nil || rows == 0`)
- Line 276: `rows, _ := result.RowsAffected()` in `StoreActivation`

**Step 2: Implement the fixes**

For line 129 in `TryAcquireLock`:

Replace:
```go
	_, _ = s.db.ExecContext(ctx, cleanQuery, key)
```
With:
```go
	if _, err := s.db.ExecContext(ctx, cleanQuery, key); err != nil {
		slog.Warn("Postgres identity: cleanup query failed",
			slog.String("key", key), slog.Any("error", err))
	}
```

For line 276 in `StoreActivation`:

Replace:
```go
	rows, _ := result.RowsAffected()
```
With:
```go
	rows, err := result.RowsAffected()
	if err != nil {
		slog.Warn("Postgres identity: RowsAffected failed",
			slog.String("key", key), slog.Any("error", err))
	}
```

**Step 3: Verify compilation**

Run: `go build ./cluster/identitylookup/postgres/`
Expected: Success

**Step 4: Run existing tests if any**

Run: `go test ./cluster/identitylookup/postgres/ -v -race`
Expected: PASS (or skip if no tests exist / need postgres)

**Step 5: Commit**

```bash
git add cluster/identitylookup/postgres/postgres_identity.go
git commit -m "fix(postgres): handle suppressed errors in identity storage

Log cleanup query and RowsAffected errors instead of silently
discarding them."
```

---

### Task 10: Add NatsKV identity nil pointer prevention

**Files:**
- Modify: `cluster/clusterproviders/natskv/natskv_identity.go`
- Modify: `cluster/clusterproviders/natskv/natskv_identity_test.go`

The `IdentityLookup.Setup()` method (line 57) silently returns when bucket creation fails, leaving `il.identities` and `il.memberTracker` nil. Any subsequent call will nil-pointer panic.

The `IdentityLookup` interface's `Setup` returns no error (signature: `Setup(cluster *Cluster, kinds []string, isClient bool)`), so we store the error and check in every public method.

**Step 1: Write the failing test**

Add to `cluster/clusterproviders/natskv/natskv_identity_test.go`:

```go
func TestIdentityLookup_SetupError_GetReturnsNil(t *testing.T) {
	// Create a provider with invalid connection to force Setup to fail
	// We can create an IdentityLookup with a nil provider.js to trigger the error

	il := &IdentityLookup{
		config:    defaultConfig("test"),
		semaphore: make(chan struct{}, 1),
	}
	// il.identities and il.memberTracker are nil (simulating failed setup)
	// Set the setupErr to simulate a failed setup
	il.setupErr = fmt.Errorf("simulated setup failure")

	// Get should return nil, not panic
	assert.NotPanics(t, func() {
		result := il.Get(&cluster.ClusterIdentity{Kind: "test", Identity: "1"})
		assert.Nil(t, result)
	})
}

func TestIdentityLookup_SetupError_RemovePidDoesNotPanic(t *testing.T) {
	il := &IdentityLookup{
		config:    defaultConfig("test"),
		semaphore: make(chan struct{}, 1),
		setupErr:  fmt.Errorf("simulated setup failure"),
	}

	assert.NotPanics(t, func() {
		il.RemovePid(&cluster.ClusterIdentity{Kind: "test", Identity: "1"}, actor.NewPID("addr", "id"))
	})
}

func TestIdentityLookup_SetupError_ShutdownDoesNotPanic(t *testing.T) {
	il := &IdentityLookup{
		config:    defaultConfig("test"),
		semaphore: make(chan struct{}, 1),
		setupErr:  fmt.Errorf("simulated setup failure"),
		memberID:  "test-member",
	}

	assert.NotPanics(t, func() {
		il.Shutdown()
	})
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./cluster/clusterproviders/natskv/ -run "TestIdentityLookup_SetupError" -v`
Expected: FAIL — compilation error because `setupErr` field doesn't exist yet

**Step 3: Implement the fix**

In `cluster/clusterproviders/natskv/natskv_identity.go`:

Add a `setupErr` field to the struct (after line 30):

```go
type IdentityLookup struct {
	provider      *Provider
	cluster       *cluster.Cluster
	memberID      string
	isClient      bool
	identities    jetstream.KeyValue
	memberTracker jetstream.KeyValue
	config        *config
	semaphore     chan struct{}
	setupErr      error
}
```

Update `Setup` to store the error (around line 71-75):

Replace:
```go
	if err != nil {
		slog.Error("natskv identity: failed to create identities bucket",
			slog.Any("error", err))
		return
	}
```
With:
```go
	if err != nil {
		il.setupErr = fmt.Errorf("natskv identity setup failed: create identities bucket: %w", err)
		slog.Error("natskv identity: failed to create identities bucket",
			slog.Any("error", err))
		return
	}
```

And for the tracker bucket creation (around line 83-86):

Replace:
```go
	if err != nil {
		slog.Error("natskv identity: failed to create tracking bucket",
			slog.Any("error", err))
		return
	}
```
With:
```go
	if err != nil {
		il.setupErr = fmt.Errorf("natskv identity setup failed: create tracking bucket: %w", err)
		slog.Error("natskv identity: failed to create tracking bucket",
			slog.Any("error", err))
		return
	}
```

Add a guard to `Get` (after the `acquire/defer release` lines, before any use of `il.identities`):

```go
func (il *IdentityLookup) Get(ci *cluster.ClusterIdentity) *actor.PID {
	if il.setupErr != nil {
		slog.Error("natskv identity: cannot Get, setup failed", slog.Any("error", il.setupErr))
		return nil
	}
	il.acquire()
	defer il.release()
	// ... rest of existing code
```

Add a guard to `RemovePid`:

```go
func (il *IdentityLookup) RemovePid(ci *cluster.ClusterIdentity, pid *actor.PID) {
	if il.setupErr != nil {
		slog.Error("natskv identity: cannot RemovePid, setup failed", slog.Any("error", il.setupErr))
		return
	}
	// ... rest of existing code
```

Add a guard to `Shutdown`:

```go
func (il *IdentityLookup) Shutdown() {
	if il.setupErr != nil {
		return
	}
	// ... rest of existing code
```

**Step 4: Run tests to verify they pass**

Run: `go test ./cluster/clusterproviders/natskv/ -run "TestIdentityLookup_SetupError" -v -race`
Expected: PASS

**Step 5: Run all natskv tests for regressions**

Run: `go test ./cluster/clusterproviders/natskv/ -v -race`
Expected: All PASS

**Step 6: Commit**

```bash
git add cluster/clusterproviders/natskv/natskv_identity.go cluster/clusterproviders/natskv/natskv_identity_test.go
git commit -m "fix(natskv): prevent nil pointer panics when identity Setup fails

Store setup error and check it in all public methods (Get, RemovePid,
Shutdown) to avoid nil-pointer panics on il.identities and
il.memberTracker."
```

---

### Task 11: Fix Prometheus silent failure

**Files:**
- Modify: `actor/config.go:62-67`
- Modify: `actor/config_test.go`

**Step 1: Write a test for the error log**

The `defaultPrometheusProvider` function at line 62 returns `nil` silently when the prometheus exporter fails to initialize. Since we can't easily make `prometheus.New()` fail in a unit test, we'll verify the function's behavior by testing that it logs correctly when it does fail. A simpler approach: just fix the code and verify it compiles and existing tests still pass. The important part is the logging.

However, we can write a test that verifies the function still returns a valid provider under normal conditions.

Add to `actor/config_test.go`:

```go
func TestDefaultPrometheusProvider_ReturnsProvider(t *testing.T) {
	// Use a random high port to avoid conflicts
	provider := defaultPrometheusProvider(19876)
	assert.NotNil(t, provider, "defaultPrometheusProvider should return a provider")
}
```

**Step 2: Implement the fix**

In `actor/config.go`, replace lines 62-68:

```go
func defaultPrometheusProvider(port int) metric.MeterProvider {
	exporter, err := prometheus.New()
	if err != nil {
		slog.Error("Failed to initialize Prometheus exporter, metrics will be disabled",
			slog.Any("error", err))
		return nil
	}
```

**Step 3: Verify compilation and tests**

Run: `go test ./actor/ -run "TestDefaultPrometheusProvider" -v`
Expected: PASS

Run: `go test ./actor/ -v -race`
Expected: All PASS

**Step 4: Commit**

```bash
git add actor/config.go actor/config_test.go
git commit -m "fix(actor): log error when Prometheus exporter initialization fails

Replace silent nil return with an slog.Error call so operators
know why metrics are disabled."
```

---

### Final Verification

After all 11 tasks are complete:

**Step 1: Run the full test suite with race detector**

```bash
go test ./... -race -count=1 2>&1 | tail -50
```

Expected: All tests PASS, no data races detected.

**Step 2: Verify no remaining panics in the changed files**

```bash
grep -n 'panic(' remote/endpoint_reader.go remote/endpoint_writer.go cluster/clusterproviders/consul/provider_actor.go
```

Expected: No panics in the fixed locations (there may be panics elsewhere that are Ring 2+ scope).

**Step 3: Verify no remaining suppressed errors in changed files**

```bash
grep -n '_, *_ *=' cluster/identitylookup/nats/nats_identity.go cluster/identitylookup/postgres/postgres_identity.go
```

Expected: No suppressed errors in the fixed locations.
