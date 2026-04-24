# Cluster Request Headers Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire message headers through `cluster.Request`/`RequestFuture` end-to-end so a caller can attach headers that reach the receiving grain.

**Architecture:** Two-layer fix — (1) in `actor/`, introduce an idempotent envelope helper used by every send path so caller-supplied `*MessageEnvelope` headers are preserved; (2) in `cluster/`, add a `WithHeaders` `GrainCallOption` plus merge logic in `DefaultContext` that attaches option-provided headers to the outgoing message, with envelope values winning on conflict. Matches .NET upstream semantics. See spec: `docs/superpowers/specs/2026-04-24-cluster-request-headers-design.md`.

**Tech Stack:** Go, `protoactor-go` actor/remote/cluster packages, `testify/assert` + `testify/require`. For cluster tests that need live grain dispatch (i.e., `cluster.Request` to an actual kind), use `newClusterForIntegrationTest()` — defined in `cluster/grain_registry_integration_test.go` — which wires an enumerable lookup and a functional in-memory provider and lets `StartMember()` succeed. `newClusterForTest()` is only for cluster plumbing tests that don't dispatch live; it does not fully boot. `-race` on all test runs.

---

## File Map

**Created:**
- None — all changes modify existing files.

**Modified:**
- `actor/message_envelope.go` — add unexported `envelopeWithSender`, exported `EnvelopeWithHeaders`.
- `actor/message_envelope_test.go` — tests for both helpers.
- `actor/root_context.go` — `RequestFuture`, `RequestWithCustomSender` use `envelopeWithSender`.
- `actor/root_context_test.go` — tests that pre-wrapped envelope headers survive through `RootContext` request methods.
- `actor/actor_context.go` — `Request`, `RequestFuture`, `RequestWithCustomSender` use `envelopeWithSender`.
- `actor/actor_context_test.go` — tests that pre-wrapped envelope headers survive through `actorContext` request methods, plus Forward regression test.
- `cluster/grain.go` — `GrainCallConfig.Headers` field, `WithHeaders` option.
- `cluster/grain_test.go` (create if absent) — unit test for `WithHeaders` option.
- `cluster/default_context.go` — apply `actor.EnvelopeWithHeaders` in `Request` and `RequestFuture` when `callConfig.Headers` is set.
- `cluster/default_context_test.go` — end-to-end header tests with a local in-memory cluster.

---

## Task 1: Actor helper — `envelopeWithSender` (unexported)

**Files:**
- Modify: `actor/message_envelope.go` (add after `WrapEnvelope`, around line 73)
- Test: `actor/message_envelope_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `actor/message_envelope_test.go`:

```go
func TestEnvelopeWithSender_RawMessage_NilSender(t *testing.T) {
	t.Parallel()

	out := envelopeWithSender("hello", nil)
	assert.NotNil(t, out)
	assert.Nil(t, out.Header)
	assert.Equal(t, "hello", out.Message)
	assert.Nil(t, out.Sender)
}

func TestEnvelopeWithSender_RawMessage_WithSender(t *testing.T) {
	t.Parallel()

	sender := NewPID("addr", "id")
	out := envelopeWithSender("hello", sender)
	assert.NotNil(t, out)
	assert.Nil(t, out.Header)
	assert.Equal(t, "hello", out.Message)
	assert.Same(t, sender, out.Sender)
}

func TestEnvelopeWithSender_Envelope_NilSender_ReturnsSame(t *testing.T) {
	t.Parallel()

	in := &MessageEnvelope{
		Header:  messageHeader{"k": "v"},
		Message: "hello",
		Sender:  NewPID("a", "1"),
	}
	out := envelopeWithSender(in, nil)
	assert.Same(t, in, out, "with nil sender, envelope should pass through unchanged")
}

func TestEnvelopeWithSender_Envelope_WithSender_ClonesAndOverrides(t *testing.T) {
	t.Parallel()

	originalSender := NewPID("orig", "1")
	in := &MessageEnvelope{
		Header:  messageHeader{"trace": "abc"},
		Message: "hello",
		Sender:  originalSender,
	}

	newSender := NewPID("new", "2")
	out := envelopeWithSender(in, newSender)

	// Clone returned, caller's envelope untouched.
	assert.NotSame(t, in, out)
	assert.Same(t, originalSender, in.Sender, "caller's envelope must not be mutated")

	// Clone has header preserved and sender replaced.
	assert.Equal(t, "abc", out.Header.Get("trace"))
	assert.Equal(t, "hello", out.Message)
	assert.Same(t, newSender, out.Sender)
}
```

- [ ] **Step 2: Run the test, expect failure**

Run: `go test ./actor/ -run TestEnvelopeWithSender -race -v`
Expected: compilation error — `undefined: envelopeWithSender`.

- [ ] **Step 3: Implement the helper**

Append to `actor/message_envelope.go` after `WrapEnvelope` (around line 73):

```go
// envelopeWithSender returns a *MessageEnvelope carrying message with the
// given sender. If message is already a *MessageEnvelope, its Header is
// preserved; when sender is non-nil the returned envelope is a copy of the
// input with Sender replaced (the caller's envelope is never mutated). If
// sender is nil, an existing envelope is returned as-is.
func envelopeWithSender(message any, sender *PID) *MessageEnvelope {
	if env, ok := message.(*MessageEnvelope); ok {
		if sender == nil {
			return env
		}
		out := *env
		out.Sender = sender
		return &out
	}
	return &MessageEnvelope{Header: nil, Message: message, Sender: sender}
}
```

- [ ] **Step 4: Run the tests, expect pass**

Run: `go test ./actor/ -run TestEnvelopeWithSender -race -v`
Expected: all four sub-tests PASS.

- [ ] **Step 5: Commit**

```bash
git add actor/message_envelope.go actor/message_envelope_test.go
git commit -m "feat(actor): add envelopeWithSender helper for idempotent envelope construction"
```

---

## Task 2: Actor helper — `EnvelopeWithHeaders` (exported)

**Files:**
- Modify: `actor/message_envelope.go` (add after `envelopeWithSender`)
- Test: `actor/message_envelope_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `actor/message_envelope_test.go`:

```go
func TestEnvelopeWithHeaders_RawMessage_EmptyHeaders(t *testing.T) {
	t.Parallel()

	out := EnvelopeWithHeaders("hello", nil)
	assert.NotNil(t, out)
	assert.Nil(t, out.Header)
	assert.Equal(t, "hello", out.Message)
	assert.Nil(t, out.Sender)
}

func TestEnvelopeWithHeaders_RawMessage_WithHeaders(t *testing.T) {
	t.Parallel()

	out := EnvelopeWithHeaders("hello", map[string]string{"k": "v"})
	assert.NotNil(t, out)
	assert.Equal(t, 1, out.Header.Length())
	assert.Equal(t, "v", out.Header.Get("k"))
	assert.Equal(t, "hello", out.Message)
}

func TestEnvelopeWithHeaders_Envelope_EmptyHeaders_ReturnsSame(t *testing.T) {
	t.Parallel()

	in := &MessageEnvelope{
		Header:  messageHeader{"k": "v"},
		Message: "hello",
	}
	out := EnvelopeWithHeaders(in, nil)
	assert.Same(t, in, out)
}

func TestEnvelopeWithHeaders_Envelope_WithHeaders_ClonesAndMerges(t *testing.T) {
	t.Parallel()

	in := &MessageEnvelope{
		Header:  messageHeader{"k": "envelope-value"},
		Message: "hello",
	}

	out := EnvelopeWithHeaders(in, map[string]string{
		"k": "option-value", // should lose to envelope
		"m": "n",            // should be added
	})

	assert.NotSame(t, in, out, "must clone to avoid mutating caller's envelope")
	assert.Equal(t, 1, in.Header.Length(), "caller's envelope must not be mutated")
	assert.Equal(t, "envelope-value", in.Header.Get("k"))

	assert.Equal(t, 2, out.Header.Length())
	assert.Equal(t, "envelope-value", out.Header.Get("k"), "envelope wins on conflict")
	assert.Equal(t, "n", out.Header.Get("m"), "non-overlapping option key added")
	assert.Equal(t, "hello", out.Message)
}

func TestEnvelopeWithHeaders_Envelope_NilHeader_AddsHeaders(t *testing.T) {
	t.Parallel()

	in := &MessageEnvelope{
		Header:  nil,
		Message: "hello",
	}

	out := EnvelopeWithHeaders(in, map[string]string{"k": "v"})

	assert.NotSame(t, in, out)
	assert.Nil(t, in.Header, "caller's envelope header must remain nil")
	assert.Equal(t, 1, out.Header.Length())
	assert.Equal(t, "v", out.Header.Get("k"))
}
```

- [ ] **Step 2: Run the test, expect failure**

Run: `go test ./actor/ -run TestEnvelopeWithHeaders -race -v`
Expected: compilation error — `undefined: EnvelopeWithHeaders`.

- [ ] **Step 3: Implement the helper**

Append to `actor/message_envelope.go` after `envelopeWithSender`:

```go
// EnvelopeWithHeaders returns a *MessageEnvelope carrying message with the
// given headers attached. If message is already a *MessageEnvelope, the
// returned envelope is a copy: existing envelope header values win on key
// conflict (explicit wrap is more specific than caller-provided headers).
// The caller's envelope is never mutated. If headers is empty, an existing
// envelope is returned as-is and a raw message is wrapped without a header
// map.
func EnvelopeWithHeaders(message any, headers map[string]string) *MessageEnvelope {
	if env, ok := message.(*MessageEnvelope); ok {
		if len(headers) == 0 {
			return env
		}
		out := &MessageEnvelope{
			Header:  make(messageHeader, len(headers)+env.Header.Length()),
			Message: env.Message,
			Sender:  env.Sender,
		}
		for k, v := range headers {
			out.Header[k] = v
		}
		for _, k := range env.Header.Keys() {
			out.Header[k] = env.Header.Get(k) // envelope wins
		}
		return out
	}
	out := &MessageEnvelope{Message: message}
	if len(headers) > 0 {
		out.Header = make(messageHeader, len(headers))
		for k, v := range headers {
			out.Header[k] = v
		}
	}
	return out
}
```

- [ ] **Step 4: Run the tests, expect pass**

Run: `go test ./actor/ -run TestEnvelopeWithHeaders -race -v`
Expected: all five sub-tests PASS.

- [ ] **Step 5: Commit**

```bash
git add actor/message_envelope.go actor/message_envelope_test.go
git commit -m "feat(actor): add EnvelopeWithHeaders for header attachment with envelope-wins merge"
```

---

## Task 3: RootContext — failing tests for header preservation

**Files:**
- Test: `actor/root_context_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `actor/root_context_test.go`. If the file has a package declaration and imports, add these inside the existing package; do not create a new imports block.

```go
// Asserts pre-wrapped envelope headers survive RequestFuture via RootContext.
func TestRootContext_RequestFuture_PreservesEnvelopeHeaders(t *testing.T) {
	t.Parallel()

	type gotHeaders struct {
		traceID string
		msg     any
		hdrLen  int
	}
	result := make(chan gotHeaders, 1)

	props := PropsFromFunc(func(ctx Context) {
		if _, ok := ctx.Message().(string); ok {
			result <- gotHeaders{
				traceID: ctx.MessageHeader().Get("trace-id"),
				msg:     ctx.Message(),
				hdrLen:  ctx.MessageHeader().Length(),
			}
			ctx.Respond("ok")
		}
	})
	a := rootContext.Spawn(props)
	defer func() { _ = rootContext.StopFuture(a).Wait() }()

	env := WrapEnvelope("hello")
	env.SetHeader("trace-id", "abc")

	f := rootContext.RequestFuture(a, env, testTimeout)
	_, err := f.Result()
	assert.NoError(t, err)

	got := <-result
	assert.Equal(t, "abc", got.traceID, "receiver must see envelope header")
	assert.Equal(t, "hello", got.msg, "receiver's ctx.Message() must be the inner payload, not the envelope")
	assert.Equal(t, 1, got.hdrLen)
}

// Asserts raw-message RequestFuture still produces empty headers (regression).
func TestRootContext_RequestFuture_RawMessage_EmptyHeaders(t *testing.T) {
	t.Parallel()

	hdrLen := make(chan int, 1)

	props := PropsFromFunc(func(ctx Context) {
		if _, ok := ctx.Message().(string); ok {
			hdrLen <- ctx.MessageHeader().Length()
			ctx.Respond("ok")
		}
	})
	a := rootContext.Spawn(props)
	defer func() { _ = rootContext.StopFuture(a).Wait() }()

	f := rootContext.RequestFuture(a, "hello", testTimeout)
	_, err := f.Result()
	assert.NoError(t, err)

	assert.Equal(t, 0, <-hdrLen)
}

// Asserts RequestWithCustomSender preserves envelope headers and overrides sender.
func TestRootContext_RequestWithCustomSender_PreservesHeadersOverridesSender(t *testing.T) {
	t.Parallel()

	type received struct {
		sender  *PID
		traceID string
		msg     any
	}
	got := make(chan received, 1)

	props := PropsFromFunc(func(ctx Context) {
		if _, ok := ctx.Message().(string); ok {
			got <- received{
				sender:  ctx.Sender(),
				traceID: ctx.MessageHeader().Get("trace-id"),
				msg:     ctx.Message(),
			}
		}
	})
	a := rootContext.Spawn(props)
	defer func() { _ = rootContext.StopFuture(a).Wait() }()

	customSender := NewPID("custom-addr", "custom-id")
	env := WrapEnvelope("hello")
	env.SetHeader("trace-id", "xyz")
	env.Sender = NewPID("other", "other") // should be overridden

	rootContext.RequestWithCustomSender(a, env, customSender)

	r := <-got
	assert.Equal(t, "xyz", r.traceID)
	assert.Equal(t, "hello", r.msg)
	assert.Equal(t, customSender.Address, r.sender.Address)
	assert.Equal(t, customSender.Id, r.sender.Id)
}
```

- [ ] **Step 2: Run the tests, expect failure**

Run: `go test ./actor/ -run "TestRootContext_RequestFuture_PreservesEnvelopeHeaders|TestRootContext_RequestWithCustomSender_PreservesHeadersOverridesSender" -race -v`
Expected: FAIL. The two "Preserves" tests fail because the current `RequestFuture`/`RequestWithCustomSender` construct a fresh envelope with `Header: nil` and drop the caller's headers. The raw-message test passes.

- [ ] **Step 3: Commit the failing tests (red)**

```bash
git add actor/root_context_test.go
git commit -m "test(actor): failing tests for RootContext envelope header preservation"
```

---

## Task 4: RootContext — implement header preservation

**Files:**
- Modify: `actor/root_context.go:134-154`

- [ ] **Step 1: Change `RequestWithCustomSender` and `RequestFuture`**

Replace the body of `RequestWithCustomSender` (around line 134):

```go
// RequestWithCustomSender sends a message on behalf of the provided sender PID.
func (rc *RootContext) RequestWithCustomSender(pid *PID, message any, sender *PID) {
	rc.sendUserMessage(pid, envelopeWithSender(message, sender))
}
```

Replace the body of `RequestFuture` (around line 144):

```go
// RequestFuture sends a message to a given PID and returns a Future.
func (rc *RootContext) RequestFuture(pid *PID, message any, timeout time.Duration) Future {
	future := NewFuture(rc.actorSystem, timeout)
	rc.sendUserMessage(pid, envelopeWithSender(message, future.PID()))

	return future
}
```

- [ ] **Step 2: Run the tests, expect pass**

Run: `go test ./actor/ -run "TestRootContext_" -race -v`
Expected: all `TestRootContext_*` pass, including the two previously-failing header tests and the regression test.

- [ ] **Step 3: Run the full `actor` package test suite**

Run: `go test ./actor/ -race`
Expected: PASS. No existing test should break.

- [ ] **Step 4: Commit**

```bash
git add actor/root_context.go
git commit -m "fix(actor): RootContext.RequestFuture/RequestWithCustomSender preserve envelope headers"
```

---

## Task 5: actorContext — failing tests for header preservation

**Files:**
- Test: `actor/actor_context_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `actor/actor_context_test.go`. These tests drive requests from inside an actor so they exercise the `actorContext` implementations:

```go
// Two-actor setup: sender actor issues Request/RequestFuture/RequestWithCustomSender
// to a receiver; assertions are about what the receiver observes.
func TestActorContext_Request_PreservesEnvelopeHeaders(t *testing.T) {
	t.Parallel()

	type received struct {
		traceID string
		msg     any
		sender  *PID
	}
	got := make(chan received, 1)

	receiverProps := PropsFromFunc(func(ctx Context) {
		if _, ok := ctx.Message().(string); ok {
			got <- received{
				traceID: ctx.MessageHeader().Get("trace-id"),
				msg:     ctx.Message(),
				sender:  ctx.Sender(),
			}
		}
	})
	receiver := rootContext.Spawn(receiverProps)
	defer func() { _ = rootContext.StopFuture(receiver).Wait() }()

	senderSelfCh := make(chan *PID, 1)
	senderProps := PropsFromFunc(func(ctx Context) {
		switch ctx.Message().(type) {
		case *Started:
			senderSelfCh <- ctx.Self()
			env := WrapEnvelope("hello")
			env.SetHeader("trace-id", "req")
			ctx.Request(receiver, env)
		}
	})
	sender := rootContext.Spawn(senderProps)
	defer func() { _ = rootContext.StopFuture(sender).Wait() }()

	senderSelf := <-senderSelfCh
	r := <-got
	assert.Equal(t, "req", r.traceID)
	assert.Equal(t, "hello", r.msg)
	assert.Equal(t, senderSelf.Id, r.sender.Id, "ctx.Request must set sender to ctx.Self()")
}

func TestActorContext_RequestFuture_PreservesEnvelopeHeaders(t *testing.T) {
	t.Parallel()

	type received struct {
		traceID string
		msg     any
	}
	got := make(chan received, 1)

	receiverProps := PropsFromFunc(func(ctx Context) {
		if _, ok := ctx.Message().(string); ok {
			got <- received{
				traceID: ctx.MessageHeader().Get("trace-id"),
				msg:     ctx.Message(),
			}
			ctx.Respond("ok")
		}
	})
	receiver := rootContext.Spawn(receiverProps)
	defer func() { _ = rootContext.StopFuture(receiver).Wait() }()

	done := make(chan struct{})
	senderProps := PropsFromFunc(func(ctx Context) {
		switch ctx.Message().(type) {
		case *Started:
			env := WrapEnvelope("hello")
			env.SetHeader("trace-id", "rf")
			f := ctx.RequestFuture(receiver, env, testTimeout)
			go func() {
				_, _ = f.Result()
				close(done)
			}()
		}
	})
	sender := rootContext.Spawn(senderProps)
	defer func() { _ = rootContext.StopFuture(sender).Wait() }()

	select {
	case <-done:
	case <-time.After(testTimeout):
		t.Fatal("future did not complete")
	}

	r := <-got
	assert.Equal(t, "rf", r.traceID)
	assert.Equal(t, "hello", r.msg)
}

func TestActorContext_RequestWithCustomSender_PreservesHeadersOverridesSender(t *testing.T) {
	t.Parallel()

	type received struct {
		traceID string
		msg     any
		sender  *PID
	}
	got := make(chan received, 1)

	receiverProps := PropsFromFunc(func(ctx Context) {
		if _, ok := ctx.Message().(string); ok {
			got <- received{
				traceID: ctx.MessageHeader().Get("trace-id"),
				msg:     ctx.Message(),
				sender:  ctx.Sender(),
			}
		}
	})
	receiver := rootContext.Spawn(receiverProps)
	defer func() { _ = rootContext.StopFuture(receiver).Wait() }()

	customSender := NewPID("cs-addr", "cs-id")
	senderProps := PropsFromFunc(func(ctx Context) {
		switch ctx.Message().(type) {
		case *Started:
			env := WrapEnvelope("hello")
			env.SetHeader("trace-id", "cs")
			env.Sender = NewPID("ignored", "ignored")
			ctx.RequestWithCustomSender(receiver, env, customSender)
		}
	})
	sender := rootContext.Spawn(senderProps)
	defer func() { _ = rootContext.StopFuture(sender).Wait() }()

	r := <-got
	assert.Equal(t, "cs", r.traceID)
	assert.Equal(t, "hello", r.msg)
	assert.Equal(t, customSender.Id, r.sender.Id)
	assert.Equal(t, customSender.Address, r.sender.Address)
}
```

If the test file does not already import `time`, add it.

- [ ] **Step 2: Run the tests, expect failure**

Run: `go test ./actor/ -run "TestActorContext_Request_PreservesEnvelopeHeaders|TestActorContext_RequestFuture_PreservesEnvelopeHeaders|TestActorContext_RequestWithCustomSender_PreservesHeadersOverridesSender" -race -v`
Expected: all three FAIL. `ctx.MessageHeader().Get("trace-id")` returns "" because the current `actorContext` methods construct envelopes with `Header: nil`.

- [ ] **Step 3: Commit the failing tests (red)**

```bash
git add actor/actor_context_test.go
git commit -m "test(actor): failing tests for actorContext envelope header preservation"
```

---

## Task 6: actorContext — implement header preservation

**Files:**
- Modify: `actor/actor_context.go:323-352`

- [ ] **Step 1: Change `Request`, `RequestWithCustomSender`, `RequestFuture`**

Replace the body of `Request` (around line 323):

```go
func (ctx *actorContext) Request(pid *PID, message any) {
	ctx.sendUserMessage(pid, envelopeWithSender(message, ctx.Self()))
}
```

Replace the body of `RequestWithCustomSender` (around line 333):

```go
func (ctx *actorContext) RequestWithCustomSender(pid *PID, message any, sender *PID) {
	ctx.sendUserMessage(pid, envelopeWithSender(message, sender))
}
```

Replace the body of `RequestFuture` (around line 342):

```go
func (ctx *actorContext) RequestFuture(pid *PID, message any, timeout time.Duration) Future {
	future := NewFuture(ctx.actorSystem, timeout)
	ctx.sendUserMessage(pid, envelopeWithSender(message, future.PID()))

	return future
}
```

- [ ] **Step 2: Run the tests, expect pass**

Run: `go test ./actor/ -run "TestActorContext_" -race -v`
Expected: all previously-failing header tests PASS.

- [ ] **Step 3: Run the full `actor` package test suite**

Run: `go test ./actor/ -race`
Expected: PASS. No existing test should break.

- [ ] **Step 4: Commit**

```bash
git add actor/actor_context.go
git commit -m "fix(actor): actorContext.Request*/RequestFuture preserve envelope headers"
```

---

## Task 7: Forward — regression test for header preservation

`actorContext.Forward` (`actor/actor_context.go:256-265`) already calls `sendUserMessage(pid, ctx.messageOrEnvelope)`, which passes the existing envelope through. It should already preserve headers and the original sender. Add a test to lock that invariant in place.

**Files:**
- Test: `actor/actor_context_test.go`

- [ ] **Step 1: Write the test**

Append to `actor/actor_context_test.go`:

```go
// Forward preserves the original envelope headers and does not overwrite the sender.
func TestActorContext_Forward_PreservesHeadersAndSender(t *testing.T) {
	t.Parallel()

	type received struct {
		traceID string
		msg     any
		sender  *PID
	}
	got := make(chan received, 1)

	originalSender := NewPID("orig-addr", "orig-id")

	finalProps := PropsFromFunc(func(ctx Context) {
		if _, ok := ctx.Message().(string); ok {
			got <- received{
				traceID: ctx.MessageHeader().Get("trace-id"),
				msg:     ctx.Message(),
				sender:  ctx.Sender(),
			}
		}
	})
	final := rootContext.Spawn(finalProps)
	defer func() { _ = rootContext.StopFuture(final).Wait() }()

	forwarderProps := PropsFromFunc(func(ctx Context) {
		if _, ok := ctx.Message().(string); ok {
			ctx.Forward(final)
		}
	})
	forwarder := rootContext.Spawn(forwarderProps)
	defer func() { _ = rootContext.StopFuture(forwarder).Wait() }()

	env := WrapEnvelope("hello")
	env.SetHeader("trace-id", "fwd")
	rootContext.RequestWithCustomSender(forwarder, env, originalSender)

	r := <-got
	assert.Equal(t, "fwd", r.traceID)
	assert.Equal(t, "hello", r.msg)
	assert.Equal(t, originalSender.Id, r.sender.Id, "Forward must preserve original sender")
}
```

- [ ] **Step 2: Run the test, expect pass**

Run: `go test ./actor/ -run TestActorContext_Forward_PreservesHeadersAndSender -race -v`
Expected: PASS (Forward already behaves correctly; this test guards against future regression).

- [ ] **Step 3: Commit**

```bash
git add actor/actor_context_test.go
git commit -m "test(actor): guard actorContext.Forward header and sender preservation"
```

---

## Task 8: Audit in-repo callers passing `*MessageEnvelope` as message

Before this fix, passing a `*MessageEnvelope` as the `message` argument to `Send`/`Request`/`RequestFuture`/`RequestWithCustomSender` silently double-wrapped: the inner envelope ended up inside the outer envelope's `Message` field. Receivers that relied on this broken shape will behave differently now. Find and audit any such callers.

**Files:** various (audit-only).

- [ ] **Step 1: Grep for potential callers**

Run each and read the results:

```bash
grep -rn "RequestFuture(.*&MessageEnvelope\|Request(.*&MessageEnvelope\|RequestWithCustomSender(.*&MessageEnvelope\|Send(.*&MessageEnvelope" --include="*.go" /home/cchamplin/development/protoactor-go | grep -v agent-vendored
grep -rn "RequestFuture(.*WrapEnvelope\|Request(.*WrapEnvelope" --include="*.go" /home/cchamplin/development/protoactor-go | grep -v agent-vendored
grep -rn "\.RequestFuture([^,]*envelope\|\.Request([^,]*envelope" --include="*.go" /home/cchamplin/development/protoactor-go | grep -v agent-vendored
```

Expected: mostly empty outside the tests just added. Ignore matches in `agent-vendored/` (C# code) and matches in tests written in tasks 3/5/7 above.

- [ ] **Step 2: For each hit (if any), determine intent**

For each hit:
- If the caller was using double-wrap intentionally (e.g., asserting `ctx.Message().(*MessageEnvelope)` on the receiver), that code is broken today and relies on a bug. Open an inline note in the task output describing what you found; do not silently change behavior that the test suite covers — update the caller to the correct pattern (unwrap before passing).
- If no hits, record "no callers found" in the commit that closes this task (or skip the commit if nothing changed).

- [ ] **Step 3: Run the whole `./...` test suite to catch anything the audit missed**

Run: `go test ./... -race`
Expected: PASS.

- [ ] **Step 4: Commit (only if changes)**

If any callers were modified:

```bash
git add <files>
git commit -m "refactor: fix callers that relied on envelope double-wrap"
```

If nothing changed, no commit is needed — proceed to Task 9.

---

## Task 9: Cluster — add `Headers` field and `WithHeaders` option

**Files:**
- Modify: `cluster/grain.go`
- Test: `cluster/grain_test.go` (create)

- [ ] **Step 1: Write the failing test**

Create `cluster/grain_test.go`:

```go
package cluster

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithHeaders_SetsHeadersOnConfig(t *testing.T) {
	t.Parallel()

	cfg := &GrainCallConfig{}
	WithHeaders(map[string]string{"trace": "abc", "tenant": "acme"})(cfg)

	assert.Len(t, cfg.Headers, 2)
	assert.Equal(t, "abc", cfg.Headers["trace"])
	assert.Equal(t, "acme", cfg.Headers["tenant"])
}

func TestWithHeaders_NilMap(t *testing.T) {
	t.Parallel()

	cfg := &GrainCallConfig{}
	WithHeaders(nil)(cfg)
	assert.Nil(t, cfg.Headers)
}
```

- [ ] **Step 2: Run the test, expect failure**

Run: `go test ./cluster/ -run TestWithHeaders -race -v`
Expected: compilation error — `undefined: WithHeaders`, `unknown field Headers`.

- [ ] **Step 3: Add the field and option**

In `cluster/grain.go`, add the `Headers` field to `GrainCallConfig` (line 9):

```go
type GrainCallConfig struct {
	RetryCount  int
	Timeout     time.Duration
	RetryAction func(n int) int
	Context     actor.SenderContext
	Headers     map[string]string
}
```

Append the option after `WithContext` (after line 63):

```go
// WithHeaders attaches the given headers to the outgoing request message.
// If the caller also passes a *actor.MessageEnvelope as the message argument,
// header values set directly on the envelope win on key conflict.
func WithHeaders(headers map[string]string) GrainCallOption {
	return func(config *GrainCallConfig) {
		config.Headers = headers
	}
}
```

- [ ] **Step 4: Run the test, expect pass**

Run: `go test ./cluster/ -run TestWithHeaders -race -v`
Expected: both sub-tests PASS.

- [ ] **Step 5: Commit**

```bash
git add cluster/grain.go cluster/grain_test.go
git commit -m "feat(cluster): add WithHeaders GrainCallOption"
```

---

## Task 10: Cluster DefaultContext — apply headers to outgoing message

**Files:**
- Modify: `cluster/default_context.go`
- Test: `cluster/default_context_test.go`

- [ ] **Step 1: Write the failing test**

Append to `cluster/default_context_test.go`:

```go
// spawns an echo kind that captures the received headers and payload
// and responds with an ack.
func newHeaderEchoCluster(t *testing.T, name string, seen chan<- headerObservation) *Cluster {
	t.Helper()
	props := actor.PropsFromFunc(func(ctx actor.Context) {
		if msg, ok := ctx.Message().(string); ok {
			seen <- headerObservation{
				traceID: ctx.MessageHeader().Get("trace-id"),
				tenant:  ctx.MessageHeader().Get("tenant"),
				msg:     msg,
				hdrLen:  ctx.MessageHeader().Length(),
			}
			ctx.Respond("ack")
		}
	})
	kind := NewKind("echo", props)
	c := newClusterForIntegrationTest(name, WithKinds(kind))
	require.NoError(t, c.StartMember())
	t.Cleanup(func() { c.Shutdown(true) })
	return c
}

type headerObservation struct {
	traceID string
	tenant  string
	msg     string
	hdrLen  int
}

func TestCluster_Request_WithHeadersOption_ReceiverSeesHeaders(t *testing.T) {
	t.Parallel()

	seen := make(chan headerObservation, 1)
	c := newHeaderEchoCluster(t, "test-headers-option", seen)

	resp, err := c.Request("id-1", "echo", "hello", WithHeaders(map[string]string{
		"trace-id": "abc",
		"tenant":   "acme",
	}))
	assert.NoError(t, err)
	assert.Equal(t, "ack", resp)

	obs := <-seen
	assert.Equal(t, "abc", obs.traceID)
	assert.Equal(t, "acme", obs.tenant)
	assert.Equal(t, "hello", obs.msg)
	assert.Equal(t, 2, obs.hdrLen)
}

func TestCluster_Request_NoHeaders_EmptyOnReceiver(t *testing.T) {
	t.Parallel()

	seen := make(chan headerObservation, 1)
	c := newHeaderEchoCluster(t, "test-headers-empty", seen)

	_, err := c.Request("id-1", "echo", "hello")
	assert.NoError(t, err)

	obs := <-seen
	assert.Equal(t, 0, obs.hdrLen)
}
```

If the imports don't already include `"github.com/asynkron/protoactor-go/actor"` and `"github.com/stretchr/testify/require"`, add them.

- [ ] **Step 2: Run the test, expect failure**

Run: `go test ./cluster/ -run "TestCluster_Request_WithHeadersOption_ReceiverSeesHeaders|TestCluster_Request_NoHeaders_EmptyOnReceiver" -race -v`
Expected: `TestCluster_Request_WithHeadersOption_ReceiverSeesHeaders` FAILS — receiver sees empty headers because `DefaultContext.Request` does not yet apply `callConfig.Headers`. `TestCluster_Request_NoHeaders_EmptyOnReceiver` should pass.

- [ ] **Step 3: Wire `EnvelopeWithHeaders` into `DefaultContext`**

In `cluster/default_context.go`, find the block in `Request` (around line 57) right after the option loop finishes and before the `_context := callConfig.Context` line. Insert:

```go
	if len(callConfig.Headers) > 0 {
		message = actor.EnvelopeWithHeaders(message, callConfig.Headers)
	}
```

Do the same in `RequestFuture` (around line 205), same location — after option loop, before `_context := callConfig.Context`.

- [ ] **Step 4: Run the test, expect pass**

Run: `go test ./cluster/ -run "TestCluster_Request_WithHeadersOption_ReceiverSeesHeaders|TestCluster_Request_NoHeaders_EmptyOnReceiver" -race -v`
Expected: both PASS.

- [ ] **Step 5: Commit**

```bash
git add cluster/default_context.go cluster/default_context_test.go
git commit -m "feat(cluster): apply WithHeaders option to outgoing request message"
```

---

## Task 11: Cluster — envelope-wins merge precedence

**Files:**
- Test: `cluster/default_context_test.go`

- [ ] **Step 1: Write the test**

Append to `cluster/default_context_test.go`:

```go
func TestCluster_Request_PrewrappedEnvelopeHeadersWinOverOption(t *testing.T) {
	t.Parallel()

	seen := make(chan headerObservation, 1)
	c := newHeaderEchoCluster(t, "test-headers-merge", seen)

	env := actor.WrapEnvelope("hello")
	env.SetHeader("trace-id", "from-envelope")
	// "tenant" is unique to the option.

	resp, err := c.Request("id-1", "echo", env, WithHeaders(map[string]string{
		"trace-id": "from-option", // should lose
		"tenant":   "acme",        // should be added
	}))
	assert.NoError(t, err)
	assert.Equal(t, "ack", resp)

	obs := <-seen
	assert.Equal(t, "from-envelope", obs.traceID, "envelope wins on conflict")
	assert.Equal(t, "acme", obs.tenant, "non-overlapping option key is added")
	assert.Equal(t, 2, obs.hdrLen)
}
```

- [ ] **Step 2: Run the test, expect pass**

Run: `go test ./cluster/ -run TestCluster_Request_PrewrappedEnvelopeHeadersWinOverOption -race -v`
Expected: PASS. The behavior is already correct (Task 2's `EnvelopeWithHeaders` implements envelope-wins); this test locks the invariant.

- [ ] **Step 3: Commit**

```bash
git add cluster/default_context_test.go
git commit -m "test(cluster): lock envelope-wins merge precedence for WithHeaders"
```

---

## Task 12: Cluster — caller's envelope is not mutated

**Files:**
- Test: `cluster/default_context_test.go`

- [ ] **Step 1: Write the test**

Append to `cluster/default_context_test.go`:

```go
func TestCluster_Request_DoesNotMutateCallerEnvelope(t *testing.T) {
	t.Parallel()

	seen := make(chan headerObservation, 1)
	c := newHeaderEchoCluster(t, "test-headers-nomutate", seen)

	env := actor.WrapEnvelope("hello")
	env.SetHeader("trace-id", "fixed")

	_, err := c.Request("id-1", "echo", env, WithHeaders(map[string]string{
		"tenant": "acme",
	}))
	assert.NoError(t, err)
	<-seen

	// Caller's envelope must be unchanged.
	assert.Equal(t, "fixed", env.GetHeader("trace-id"))
	assert.Equal(t, "", env.GetHeader("tenant"), "option header must not have been written into caller's envelope")
	assert.Equal(t, 1, env.Header.Length())
}
```

- [ ] **Step 2: Run the test, expect pass**

Run: `go test ./cluster/ -run TestCluster_Request_DoesNotMutateCallerEnvelope -race -v`
Expected: PASS (`EnvelopeWithHeaders` copy-on-write path is already correct; this test guards against regression).

- [ ] **Step 3: Commit**

```bash
git add cluster/default_context_test.go
git commit -m "test(cluster): guard that WithHeaders does not mutate caller's envelope"
```

---

## Task 13: Cluster — pre-wrapped envelope without `WithHeaders` still flows

**Files:**
- Test: `cluster/default_context_test.go`

- [ ] **Step 1: Write the test**

This proves the actor-core fix alone is enough — even without the cluster option.

Append to `cluster/default_context_test.go`:

```go
func TestCluster_Request_PrewrappedEnvelope_HeadersFlowWithoutOption(t *testing.T) {
	t.Parallel()

	seen := make(chan headerObservation, 1)
	c := newHeaderEchoCluster(t, "test-headers-noopt", seen)

	env := actor.WrapEnvelope("hello")
	env.SetHeader("trace-id", "raw-env")

	_, err := c.Request("id-1", "echo", env)
	assert.NoError(t, err)

	obs := <-seen
	assert.Equal(t, "raw-env", obs.traceID)
	assert.Equal(t, "hello", obs.msg)
}
```

- [ ] **Step 2: Run the test, expect pass**

Run: `go test ./cluster/ -run TestCluster_Request_PrewrappedEnvelope_HeadersFlowWithoutOption -race -v`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add cluster/default_context_test.go
git commit -m "test(cluster): pre-wrapped envelope headers flow without WithHeaders option"
```

---

## Task 14: Remote hand-off guard — `UnwrapEnvelope` extracts headers correctly

The existing in-repo test harness (`inmemoryProvider` + `fakeIdentityLookup`, and the integration variant in `grain_registry_integration_test.go`) spawns grains locally only — it does not exercise the cross-member remote wire path. A true cross-member test needs a richer harness that is out of scope for this plan.

What we can verify directly is the glue point: `remote/remote_process.go:22` calls `actor.UnwrapEnvelope(message)` to hand the envelope to the wire serializer. If our `EnvelopeWithHeaders` output unwraps cleanly into `(Header, Message, Sender)`, the rest of the wire path (already exercised daily by existing envelope traffic) handles it. This task adds a narrow guard.

**Files:**
- Test: `actor/message_envelope_test.go`

- [ ] **Step 1: Write the test**

Append to `actor/message_envelope_test.go`:

```go
// Guards the glue between the cluster-side envelope construction and the
// remote outbound path: remote/remote_process.go:22 calls UnwrapEnvelope on
// the outgoing envelope and hands header/message/sender to SendMessage. Any
// envelope built by EnvelopeWithHeaders must round-trip through UnwrapEnvelope
// with headers intact.
func TestEnvelopeWithHeaders_UnwrapsCleanlyForRemote(t *testing.T) {
	t.Parallel()

	env := EnvelopeWithHeaders("hello", map[string]string{"trace-id": "x"})

	header, msg, sender := UnwrapEnvelope(env)
	assert.Equal(t, "x", header.Get("trace-id"))
	assert.Equal(t, "hello", msg, "payload must be the inner message, not the envelope")
	assert.Nil(t, sender)
}

// Same guard when the cluster layer merges option headers with a caller's
// pre-wrapped envelope: UnwrapEnvelope must see the merged header map.
func TestEnvelopeWithHeaders_PrewrappedMergeUnwrapsCleanly(t *testing.T) {
	t.Parallel()

	in := WrapEnvelope("hello")
	in.SetHeader("trace-id", "envelope-wins")

	env := EnvelopeWithHeaders(in, map[string]string{
		"trace-id": "should-lose",
		"tenant":   "acme",
	})

	header, msg, _ := UnwrapEnvelope(env)
	assert.Equal(t, "envelope-wins", header.Get("trace-id"))
	assert.Equal(t, "acme", header.Get("tenant"))
	assert.Equal(t, "hello", msg)
}
```

- [ ] **Step 2: Run the test, expect pass**

Run: `go test ./actor/ -run "TestEnvelopeWithHeaders_UnwrapsCleanlyForRemote|TestEnvelopeWithHeaders_PrewrappedMergeUnwrapsCleanly" -race -v`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add actor/message_envelope_test.go
git commit -m "test(actor): guard EnvelopeWithHeaders output unwraps cleanly for remote path"
```

---

## Task 15: Final full-suite verification

**Files:** none

- [ ] **Step 1: Run the full repo test suite with race detection**

Run: `go test ./... -race -timeout 180s`
Expected: PASS across all packages.

- [ ] **Step 2: Run `go vet`**

Run: `go vet ./...`
Expected: no output.

- [ ] **Step 3: Verify no lingering placeholders in touched files**

Run: `git diff --stat main..HEAD` (substitute actual base branch if different from `main`/`dev`)
Inspect the diff, confirm no stray `TODO`/`FIXME` introduced for this change.

- [ ] **Step 4: Document follow-ups**

Two items flagged for separate plans, not this one:

1. **PubSub header coverage.** Since `GrainCallConfig.Headers` now exists, `cluster.Publisher.Publish` and `PublishBatch` inherit the option. A follow-up plan can add dedicated PubSub tests.
2. **Cross-member end-to-end header test.** The current harness (`inmemoryProvider` + `fakeIdentityLookup`) spawns grains locally; it does not exercise the remote wire path. A real cross-member test needs a multi-member harness (e.g., two `remote.Remote` instances on distinct ports with a shared discovery path). That harness is a larger change of its own.

Mention both in the final PR description but do not commit anything here.

---

## Checklist Summary

When all tasks are complete you should have:

- `actor/message_envelope.go` — added `envelopeWithSender` (unexported) and `EnvelopeWithHeaders` (exported).
- `actor/root_context.go` — `RequestFuture`, `RequestWithCustomSender` go through the helper.
- `actor/actor_context.go` — `Request`, `RequestFuture`, `RequestWithCustomSender` go through the helper. `Forward` unchanged (already correct).
- `cluster/grain.go` — `GrainCallConfig.Headers` + `WithHeaders` option.
- `cluster/default_context.go` — `Request`/`RequestFuture` apply `actor.EnvelopeWithHeaders` when `callConfig.Headers` is set.
- Tests covering: both helpers; both `RootContext` and `actorContext` request methods; Forward invariant; `WithHeaders` option; envelope-wins merge; no-mutation; raw-message regression; pre-wrapped no-option flow; remote hand-off glue (`UnwrapEnvelope`).
- Deferred to follow-up plans: PubSub header coverage; cross-member end-to-end test (needs new multi-member harness).
