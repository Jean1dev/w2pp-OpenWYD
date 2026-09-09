## Context

See proposal.md for motivation. The initial working tree and diff were clean and no issue-309 artifacts existed; the change was scaffolded without replacing prior work.

Evidence consulted:
- `Source/Code/TMSrv/GetFunc.cpp:782`, especially the five floor 1/2 branches around lines 871–900: exact origins, destinations, default zero Charge and two `rand()%3` calls.
- `Source/Code/TMSrv/_MSG_ReqTeleport.cpp`: routing uses authoritative TargetX/Y.
- `tmserver/internal/world/teleport.go`: only a subset of the legacy table exists; none of those five branches is represented. Lookup rounds each coordinate down to a multiple of four.
- `tmserver/internal/handler/movement.go`: `reqTeleport` already handles play/liveness, lookup and charging; `doTeleport` updates position and reconciles visibility.
- `docs/migration/handlers/lote2-movimento.md`, existing kingdom teleport spec, project overview, dependency report, TMSrv-Core analysis, AGENTS.md and Go guidelines establish protocol and ownership constraints. Kingdom chat travel is a separate capability.

The issue attachment could not be fetched through either the web reader or HTTP download. No screenshot coordinates or reproduced client symptoms are claimed. Missing routes are a concrete static defect sufficient to plan this bounded correction; the reported intermittency is not established by inspection.

## Goals / Non-Goals

**Goals:** Restore the five routes through the existing lookup and teleport pipeline; prevent partial coverage of adjacent blocks and verify wire-visible results.

**Non-Goals:** Broader table completion (including floor 3 and Underworld), new movement rules, tax fixes, client patches, map editing, timers, retries, or RNG migration. The existing alternate Armia entrance uses a spread differing from legacy; that separate discrepancy is excluded.

## Decisions

1. Add five explicit `teleRoute` entries in `world/teleport.go`, grouped and attributed to the legacy floor branches. Preserve the exact asymmetric return to (148,3780). A larger proximity rectangle could activate unrelated tiles; porting every omitted route would expand scope.
2. Reuse `TeleportDest`, `reqTeleport` and `doTeleport` unchanged unless a new focused regression demonstrates a necessary correction. No locks or asynchronous world mutation are introduced. No new protocol representation is needed.
3. Preserve the existing two destination draws (X then Y) and +0..2 spread. The current resolver uses math/rand; this change does not claim deterministic MSVC RNG parity and does not refactor other callers or consume extra random draws.
4. Add independent expected-coordinate tests, not expectations derived from the production table. World tests cover the Cartesian product of offsets 0..3 for all five blocks, landing bounds, zero cost, and nearby unregistered blocks. In particular, (147,3780) and (148,3780) both work; X 143 and 152 at Y 3780 do not become portals.
5. Handler tests dispatch opcode 0x0290 with empty payload in a world large enough for coordinates near 4096 (do not reuse the 16-tile movement fixture). Use existing in-memory fixtures, execute mutations under the established single-owner test pattern, and decode the outgoing jump to compare against authoritative position. Cover all five routes, zero and positive gold, dead/non-play rejection, unregistered position, and an explicit entry/return/re-entry sequence. Include old/new view observers to verify normal visibility reconciliation. Do not require exact random samples or probabilistic distribution checks.

## Risks / Trade-offs

- Unobserved screenshot/client behavior may involve another failure → keep the confirmed route defect distinct from the unproven intermittent symptom; optionally confirm the passages with the unmodified client after apply.
- Adjacent origins and randomized landings can conceal incomplete mappings → exhaustively test origin offsets and assert bounds for each landing; every landing remains inside a valid return block.
- Overly broad cleanup can affect other travel or RNG consumers → only add the five entries and focused tests; retain existing city route tests.

## Migration Plan

No schema, configuration or data migration is needed. After explicit apply, the orchestrator runs `make test`, `make build`, and `make vet` inside Docker. An additional focused repeated race run is specified in tasks.md. No external services or credentials are needed. Publication and deployment belong to the orchestrator; reverting the five route additions restores previous behavior if rollback is needed.
