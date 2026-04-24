---
title: Cluster Request Headers — End-to-End Support
date: 2026-04-24
status: approved
---

# Cluster Request Headers — End-to-End Support

## Problem Statement

`cluster.Request` / `cluster.RequestFuture` cannot attach message headers that reach the receiving grain. The failure is in the actor core, not the cluster or remote layers:

- `cluster/grain.go` — `GrainCallConfig` has no `Headers` field and no `WithHeaders` option.
- `actor/root_context.go:146` — `RootContext.RequestFuture` unconditionally constructs `&MessageEnvelope{Header: nil, Message: message, Sender: future.PID()}`.
- `actor/actor_context.go:344` — `actorContext.RequestFuture` has the same pattern.
- Same pattern repeats in `Request`, `RequestWithCustomSender` on both contexts.
- `RootContext.WithHeaders(headers)` (`actor/root_context.go:50`) stores `rc.headers` but it is only ever read by `MessageHeader()`. Never consulted when building an outgoing envelope.

Consequences:

1. Passing a pre-wrapped `*MessageEnvelope` as `message` **double-wraps**: the user's envelope ends up in the outer envelope's `Message` field. Receiver's `ctx.MessageHeader()` returns nil; `ctx.Message()` returns the inner envelope, not the payload.
2. `RootContext.WithHeaders(...)` is effectively dormant for sending.
3. The remote wire format already handles headers (`remote/remote_process.go:22` `UnwrapEnvelope` → `endpoint_writer.go:238` `MessageHeader{HeaderData: ...}`). The break is entirely local-side envelope construction.

For contrast, the vendored .NET upstream (`agent-vendored/protoactor-dotnet/`) carries caller-supplied headers end-to-end because `MessageEnvelope.Wrap`/`WithSender` are idempotent — a `MessageEnvelope` passed as the message argument is preserved, its `Header` intact, only its `Sender` replaced. See `src/Proto.Actor/Messages/MessageEnvelope.cs:54,80` and `src/Proto.Actor/Context/RootContext.cs:103`.

## Goals

- Preserve caller-supplied `*MessageEnvelope` headers through every actor send path: `Send`, `Request`, `RequestFuture`, `RequestWithCustomSender`, `Forward` — on both `RootContext` and `actorContext`.
- Introduce a cluster-layer `WithHeaders(map[string]string) GrainCallOption` so callers of `cluster.Request*` can attach headers without manually constructing a `MessageEnvelope`.
- Keep the change behaviorally equivalent for callers that do not use headers. No receiver that today sees `MessageHeader().Length() == 0` should see anything different.
- Match .NET semantics so developers reading both implementations see the same rules.

## Non-Goals

- Auto-propagating a context's ambient headers (`rc.headers`, `ctx.MessageHeader()`) into outgoing envelopes. .NET does not do this; we will not either. `WithHeaders` on `RootContext` remains a queryable field available to middleware.
- Dedicated PubSub header-propagation tests. `cluster.Publisher` internally invokes `RequestFuture`, so `GrainCallConfig.Headers` flows automatically; no separate API surface.
- Any change to the serialized wire format. The existing `remote.MessageHeader` proto is sufficient.

## Design Overview

Two changes, in two packages:

1. **`actor` package** — route every envelope construction in the send paths through a single idempotent helper (`envelopeWithSender`) that preserves existing envelope headers and applies the correct sender override per send semantics.
2. **`cluster` package** — add `Headers map[string]string` to `GrainCallConfig`, a `WithHeaders(map[string]string) GrainCallOption`, and merge logic in `DefaultContext.Request`/`RequestFuture` that attaches option headers to the outgoing message with **envelope values winning on conflict**.

## Detailed Design

### 1. Actor-Core Helper

New unexported helper in `actor/message_envelope.go`:

```go
// envelopeWithSender returns an envelope carrying `message`, overriding the
// sender to `sender`. If `message` is already a *MessageEnvelope, its Header
// is preserved; when `sender` is non-nil, the returned envelope is a copy of
// the input with Sender replaced (the caller's envelope is never mutated).
// If `sender` is nil, an existing envelope is returned as-is.
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

Copy-on-write: the helper never mutates an envelope the caller handed us.

### 2. Call-Site Changes

| Method | Today | After |
|---|---|---|
| `RootContext.Send` | `sendUserMessage(pid, message)` → `WrapEnvelope` (idempotent) | unchanged |
| `RootContext.Request` | `sendUserMessage(pid, message)` → `WrapEnvelope` | unchanged |
| `RootContext.RequestWithCustomSender` | fresh `&MessageEnvelope{Header: nil, Message, Sender}` | `sendUserMessage(pid, envelopeWithSender(message, sender))` |
| `RootContext.RequestFuture` | fresh envelope | `sendUserMessage(pid, envelopeWithSender(message, future.PID()))` |
| `actorContext.Send` | `WrapEnvelope` | unchanged |
| `actorContext.Request` | fresh envelope with `Sender: ctx.Self()` | `sendUserMessage(pid, envelopeWithSender(message, ctx.Self()))` |
| `actorContext.RequestWithCustomSender` | fresh envelope | `sendUserMessage(pid, envelopeWithSender(message, sender))` |
| `actorContext.RequestFuture` | fresh envelope | `sendUserMessage(pid, envelopeWithSender(message, future.PID()))` |
| `actorContext.Forward` | audit during implementation | preserves original sender (forward semantics) |

Paths that already go through `WrapEnvelope` (`Send`, `Request`) need no code change — `WrapEnvelope` is already idempotent. They are listed for completeness.

### 3. Sender Override Invariants

| Method | Sender on outgoing envelope |
|---|---|
| `Send` | From envelope if wrapped; else nil. **Never overridden**. |
| `actorContext.Request` | Always `ctx.Self()`. |
| `RootContext.Request` | Always nil (root has no self). |
| `RequestWithCustomSender` | Always the explicit `sender` argument. |
| `RequestFuture` | Always `future.PID()`. **Response delivery depends on this.** |
| `Forward` | Preserves the originating sender. |

Rationale: `RequestFuture` must own the sender because the response is routed to the future. `Request` needs the sender set so the receiver can reply. `Send` is tell-and-forget — the sender is informational and the caller's choice should stand.

### 4. Cluster-Layer Additions

**`cluster/grain.go`:**

```go
type GrainCallConfig struct {
    RetryCount  int
    Timeout     time.Duration
    RetryAction func(n int) int
    Context     actor.SenderContext
    Headers     map[string]string // NEW
}

func WithHeaders(headers map[string]string) GrainCallOption {
    return func(config *GrainCallConfig) {
        config.Headers = headers
    }
}
```

**Header construction constraint.** `MessageEnvelope.Header` has type `messageHeader` (unexported) — code outside the `actor` package cannot construct one directly. The merge helper therefore lives in the `actor` package as an exported function, and the cluster layer calls it.

**New exported helper in `actor/message_envelope.go`:**

```go
// EnvelopeWithHeaders returns a *MessageEnvelope carrying `message` with
// `headers` attached. If `message` is already a *MessageEnvelope, the returned
// envelope is a copy: existing envelope header values win on key conflict
// (explicit wrap is more specific than the caller-provided headers). The
// caller's envelope is never mutated. If `headers` is empty, an existing
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

**`cluster/default_context.go`:** in both `Request` and `RequestFuture`, after `callConfig` is populated and before the `_context.RequestFuture(pid, message, ttl)` call, apply:

```go
if len(callConfig.Headers) > 0 {
    message = actor.EnvelopeWithHeaders(message, callConfig.Headers)
}
```

When no headers are set, `message` passes through untouched and the existing fast path (no envelope allocation) is preserved.

### 5. Data Flow (Headers Present)

```
User: cluster.Request("id", "Kind", msg, cluster.WithHeaders({"trace":"x"}))
  │
  ▼
cluster.DefaultContext.Request
  - builds GrainCallConfig (Headers={"trace":"x"})
  - applyCallHeaders(msg, headers) → *MessageEnvelope{Header:{trace:x}, Message:msg, Sender:nil}
  │
  ▼
actor.(*RootContext).RequestFuture(pid, envelope, ttl)
  - envelopeWithSender(envelope, future.PID()) → clone with Sender=futurePID, Header preserved
  - rc.sendUserMessage(pid, env)
  │
  ▼
pid.sendUserMessage(...) → dispatch
  │
  ├── LOCAL: mailbox → actorContext.Receive(envelope) → ctx.MessageHeader() sees {trace:x}
  │
  └── REMOTE: remote.(*process).SendUserMessage
        - actor.UnwrapEnvelope(envelope) → (Header, Message, Sender)
        - remote.SendMessage(pid, Header, Message, Sender, -1)
        - endpoint_writer serializes → wire MessageHeader{HeaderData:{trace:x}}
        - receiver endpoint_reader reconstructs envelope with Header
        - receiver actor sees ctx.MessageHeader() = {trace:x}
```

## Edge Cases

**Caller holds a reference to their envelope after sending.** Both the actor-core helper and the cluster merge path are copy-on-write. The caller's envelope is never observed to have had its sender or headers changed.

**Forwarding received headers.** `ctx.Message()` returns the unwrapped payload, so `ctx.Send(other, ctx.Message())` drops headers — same as today, same as .NET. Users who want to propagate headers use `ctx.Forward(other)` or explicitly re-wrap using `ctx.MessageHeader()`. This is documented behavior, no change.

**`*MessageEnvelope` passed as `message` before this change.** Previously double-wrapped: receiver's `ctx.Message()` returned the inner envelope (not its payload), `ctx.MessageHeader()` was nil. After this change, an inner envelope flows through as the real envelope. Behavior change for any code that relied on the broken double-wrap. Implementation step will grep for this pattern in the repo; expectation is zero hits, since the double-wrap was always broken.

**Conflict between cluster `WithHeaders` and an explicitly wrapped envelope.** Envelope wins on conflicting keys; non-overlapping keys from the option are added. Rationale: the envelope is the more specific, explicit source.

**Empty / nil headers.** `applyCallHeaders` short-circuits when `len(headers) == 0`. `envelopeWithSender` leaves `Header` at whatever the caller had (nil or set). Downstream `remote/endpoint_writer.go:235` already treats nil and empty as equivalent.

**PubSub.** `cluster.Publisher.Publish` / `PublishBatch` take `...GrainCallOption`; they invoke `RequestFuture` internally. Once `GrainCallConfig.Headers` exists, it flows for free. No dedicated API or test coverage in this change — flagged for possible follow-up.

## Testing Strategy

**Actor-core (`actor/` package):**

1. Direct unit tests for `envelopeWithSender`:
   - Raw message, nil sender → fresh envelope, `Header: nil`.
   - Raw message, non-nil sender → fresh envelope, sender set.
   - Pre-wrapped envelope with headers, nil sender → returned as-is, headers preserved.
   - Pre-wrapped envelope with headers, non-nil sender → clone returned, sender replaced, original envelope unchanged (assert original's sender still equals what it had before the call).

2. `RootContext.RequestFuture` / `RequestWithCustomSender` integration (using a local receiver actor):
   - Pass a `*MessageEnvelope` with headers → receiver's `ctx.MessageHeader().Get("k")` equals the sent value; `ctx.Message()` equals the inner payload (not the envelope).
   - Pass a raw message → receiver sees `ctx.MessageHeader().Length() == 0` (regression).

3. `actorContext.Request` / `RequestFuture` / `RequestWithCustomSender` integration:
   - Same header tests as above.
   - Sender-of-record: on `Request` from actor A, receiver sees `ctx.Sender()` equal to A's self PID even if caller wrapped an envelope with a different sender. On `RequestFuture`, sender equals the future's PID.

**Cluster (`cluster/` package):**

1. `cluster.Request(..., cluster.WithHeaders({k:v}))` with raw message → receiver sees header.
2. `cluster.Request(..., pre-wrapped env with k:v1, cluster.WithHeaders({k:v2, m:n}))` → receiver sees `{k:v1, m:n}` (envelope wins; option adds non-overlapping).
3. After (2), assert the caller's original envelope still has `{k:v1}` only — not mutated.
4. `cluster.Request(..., pre-wrapped env with headers)` and no `WithHeaders` option → receiver still sees headers (actor-core fix alone suffices).
5. No headers anywhere → receiver `ctx.MessageHeader().Length() == 0` (regression).

**Remote integration:** one end-to-end test using the existing two-member cluster harness (`newClusterForTest`) that asserts a header set via `cluster.WithHeaders` survives a cross-member request. The wire path is already exercised — this test guards future regressions at the glue.

All new and changed tests run with `-race`.

## Rollout and Compatibility

- Public API additions only: `cluster.WithHeaders`, `GrainCallConfig.Headers`, no removals.
- Internal helper `envelopeWithSender` is unexported.
- No wire format change.
- No CLAUDE.md / external contract changes.
- Any in-repo test or sample that passed a `*MessageEnvelope` as the `message` argument (relying on double-wrap) will be flagged during the implementation grep. Expectation: no hits.

## Risks

1. **Undetected callers of the double-wrap pattern.** Mitigation: grep for `RequestFuture(.*MessageEnvelope`, `Request(.*&MessageEnvelope`, etc. Low risk — the old behavior was always broken at the receiver side.
2. **Middleware that inspected `envelope.Header` and assumed always-nil.** Low risk — middleware that reads headers on an envelope that could only ever be nil has no reason to exist. Audit any sender middleware in-repo.
3. **Forward semantics divergence.** `Forward` is not heavily covered by existing tests; we need to explicitly verify the sender-preservation invariant.

## References

- `actor/message_envelope.go` — current envelope type and helpers.
- `actor/root_context.go:129-154` — current `Request`/`RequestFuture` implementations.
- `actor/actor_context.go:323-352` — actor-side equivalents.
- `cluster/grain.go` — `GrainCallConfig` / `GrainCallOption`.
- `cluster/default_context.go:51-264` — `DefaultContext.Request`/`RequestFuture`.
- `remote/remote_process.go:21-24` — `UnwrapEnvelope` on outbound.
- `remote/endpoint_writer.go:235-240` — wire header serialization.
- `remote/endpoint_reader.go:256-266` — wire header reconstruction.
- `agent-vendored/protoactor-dotnet/src/Proto.Actor/Messages/MessageEnvelope.cs:54,80` — .NET idempotent `Wrap`/`WithSender`.
- `agent-vendored/protoactor-dotnet/src/Proto.Actor/Context/RootContext.cs:103` — .NET `Request` using `WithSender`.
- `agent-vendored/protoactor-dotnet/src/Proto.Cluster/DefaultClusterContext.cs:128` — `context.Request(pid, message, future.Pid)` in the cluster send path.
