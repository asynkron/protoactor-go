# Ring 1: Core Safety - Implementation Plan Prompt

> Use this prompt in a new Claude Code session to generate a step-by-step implementation plan using the `writing-plans` skill.

## Prompt

Use the superpowers:writing-plans skill to create a detailed implementation plan from the design document at `docs/plans/2026-02-23-ring1-core-safety-design.md`.

This is Ring 1 of a 4-ring "Concentric Rings" production hardening effort for protoactor-go. Ring 1 eliminates all code paths that crash the process or silently corrupt state.

### Important context not in the design doc:

1. **Proto definitions for remote RPCs:** Check `remote/remote.proto` for the exact message definitions of `ListProcessesRequest`, `ListProcessesResponse`, `GetProcessDiagnosticsRequest`, `GetProcessDiagnosticsResponse`, and `ConnectRequest_ClientConnection`. The implementation must match these proto schemas exactly.

2. **ProcessRegistry API:** Check `actor/process_registry.go` for how to enumerate processes. The registry uses a `sync.Map` for local PIDs. There may or may not be a `GetAllPIDs()` or `Range()` method -- inspect the actual code.

3. **ClientConnection vs ServerConnection:** In `remote/endpoint_reader.go`, the `Receive` method starting at line 42 handles `ConnectRequest`. Look at how `ServerConnection` is handled (lines ~89-113) -- `ClientConnection` should follow the same pattern but with client-appropriate semantics. The key question is: does a client connection need to be added to the endpoint manager's connection tracking?

4. **NatsKV IdentityLookup interface:** The `IdentityLookup` interface (`cluster/identity.go`) has `Setup(cluster *Cluster, kinds []string, isClient bool)` with no error return. The design recommends storing the error and checking it in each public method. Verify that ALL public methods (`Get`, `SpawnActivation`, `Shutdown`, etc.) are guarded.

5. **Test patterns in this repo:** Tests use `testify/assert` and `testify/require`. Integration tests for NATS use `testcontainers`. Unit tests for cluster use `newClusterForTest()` and `newInmemoryProvider()` from `cluster/cluster_test.go`.

6. **Existing prior work:** The 2026-02-16 production readiness effort already completed. See `docs/plans/complete/2026-02-16-cluster-production-readiness.md` for what's done. Do NOT re-implement any of that work. Ring 1 covers ONLY the items listed in the design doc.

7. **Consul provider is NOT in scope** for full production readiness -- it's included only because `provider_actor.go` has a panic that needs fixing. Don't add consul tests beyond what's needed for the panic fix.

8. **Use TDD approach:** Write a failing test for each fix before implementing it. The repo already has good test patterns to follow.

### Expected output:

A plan saved to `docs/plans/2026-02-23-ring1-core-safety.md` with:
- One task per fix item (11 items in the design)
- Each task has: files to modify, step-by-step instructions, test-first approach, specific commit message
- Tasks ordered by dependency (endpoint_reader methods first since they're the most work, then simpler fixes)
- Clear run/verify commands for each step
