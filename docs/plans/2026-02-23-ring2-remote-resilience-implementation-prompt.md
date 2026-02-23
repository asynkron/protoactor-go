# Ring 2: Remote Resilience - Implementation Plan Prompt

> Use this prompt in a new Claude Code session to generate a step-by-step implementation plan using the `writing-plans` skill.

## Prompt

Use the superpowers:writing-plans skill to create a detailed implementation plan from the design document at `docs/plans/2026-02-23-ring2-remote-resilience-design.md`.

This is Ring 2 of a 4-ring "Concentric Rings" production hardening effort for protoactor-go. Ring 2 makes the remote package self-healing after failures. **Ring 1 must be complete before starting Ring 2** -- specifically the endpoint writer panic fix at `remote/endpoint_writer.go:358-360`.

### Important context not in the design doc:

1. **Existing retry config:** Check `remote/config.go` for current config fields. `RetryBaseDelay` and `MaxRetryCount` already exist. You're adding `RetryMaxDelay`, `SupervisorRestartWindow`, and `SupervisorMaxRestarts`. Follow the existing config pattern using functional options (`WithXxx` functions).

2. **RestartStatistics API:** Check `actor/supervision.go` or `actor/restart_statistics.go` for the `RestartStatistics` type. The `NumberOfFailures(window time.Duration)` method returns the count of failures within the time window. Verify the exact method signature before writing the supervisor code.

3. **Endpoint supervisor spawning:** In `remote/endpoint_manager.go`, the supervisor spawns both an `endpointWriter` and `endpointWatcher` for each remote address. When the supervisor restarts the writer, it must NOT restart the watcher (or vice versa). Check how `RestartChildren` works -- does it restart all children or just the failed one? The `child *actor.PID` parameter in `HandleFailure` identifies which specific child failed.

4. **Graceful shutdown ordering:** The design moves `edpManager.stop()` after `GracefulStop()`. Before making this change, read `remote/endpoint_manager.go` to understand what `stop()` does. If it cancels contexts that in-flight gRPC handlers depend on, moving it after GracefulStop could cause those handlers to error. Test this carefully.

5. **Blocked status proto field:** Check the `ConnectResponse` message in `remote/remote.proto` to find the exact field name for blocked status. It may be `blocked`, `member_id`, or encoded differently. The endpoint_writer's `initializeInternal()` method at line ~85-140 processes the ConnectResponse.

6. **Server test infrastructure:** The old tests in `remote/server_test.go` used mock processes and internal field access. The new tests should be black-box. Use `actor.NewActorSystem()` and `remote.NewRemote()` with port 0 for random port selection. Check if there's a `remote.Configure()` helper or if you need to build config manually.

7. **Dead code at endpoint_writer.go:68-78:** This is the commented-out retry-after-failure code block after the `return` on line 66. All of it is unreachable. Remove it entirely when implementing exponential backoff -- the new backoff logic replaces its intent.

8. **math/rand vs crypto/rand:** Use `math/rand` for jitter calculation. Import `math/rand/v2` if the project uses Go 1.22+, otherwise use `math/rand`. Check `go.mod` for the Go version.

### Expected output:

A plan saved to `docs/plans/2026-02-23-ring2-remote-resilience.md` with:
- One task per design item (6 items)
- Tasks ordered: exponential backoff first (independent), then supervisor restart (depends on backoff), then shutdown, then blocked status, then tests, then dead code cleanup
- Each task has: files to modify, step-by-step with TDD, specific commit message
- Integration test task at the end that verifies the full retry + restart + shutdown flow
