# Shared Placement Actor — Project Tracker

**Spec:** `docs/superpowers/specs/2026-03-17-shared-placement-actor-design.md`
**Created:** 2026-03-17
**Branch:** `dev`

## Approach

The spec has 8 sub-projects (1a, 1b, 1c, 2, 3, 4, 5, 6). Each sub-project gets its own implementation plan and is independently shippable. Plans are written in batches:

- **Batch 1:** Sub-projects 1a, 1b, 1c (core components — placement actor, strategies, proxy)
- **Batch 2:** Sub-projects 2, 3, 4 (provider integrations — IdentityStorageLookup, natskv, natsstream)
- **Batch 3:** Sub-projects 5, 6 (disthash refactor, conformance + cross-node safety)

Each batch: write plans → execute → verify → then write next batch's plans.

## Sub-project Status

| Sub-project | Plan File | Plan Status | Execution Status |
|-------------|-----------|-------------|------------------|
| 1a: Foundation (proto, Kind, utilities) | `2026-03-17-sp1a-foundation.md` | Not started | Not started |
| 1b: ActivatorStrategy + 4 implementations | `2026-03-17-sp1b-strategies.md` | Not started | Not started |
| 1c: Placement Actor + Activator Proxy | `2026-03-17-sp1c-placement-actor.md` | Not started | Not started |
| 2: Integrate into IdentityStorageLookup | `2026-03-17-sp2-storage-lookup.md` | Not started | Not started |
| 3: Integrate into natskv | `2026-03-17-sp3-natskv.md` | Not started | Not started |
| 4: Integrate into natsstream | `2026-03-17-sp4-natsstream.md` | Not started | Not started |
| 5: Refactor disthash | `2026-03-17-sp5-disthash.md` | Not started | Not started |
| 6: Conformance + Cross-Node Safety | `2026-03-17-sp6-conformance.md` | Not started | Not started |

## Dependencies

```
1a → 1b → 1c → 2 → 3, 4 (parallel)
                1c → 5
                2, 3, 4, 5 → 6
```

## Prior Work in This Session

Before this project started, we completed the RemovePid liveness fix:
- Plan: `docs/superpowers/plans/2026-03-16-removepid-liveness-fix.md`
- 9 commits on `dev` branch (commits `7ef97cb0` through `32e0b50d`)
- Three-layer defense: DefaultContext timeout/deadletter split, natskv/natsstream liveness guards, ErrNameExists recovery
- All unit + integration tests pass
- Flaky `TestMailboxUserMessageCount` also fixed
