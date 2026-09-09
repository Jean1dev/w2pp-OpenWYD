## 1. Restore Dungeon floor routes

- [x] 1.1 Add the five floor 1/floor 2 entries specified in `specs/dungeon-floor-teleports/spec.md` to `tmserver/internal/world/teleport.go`, with legacy attribution; verify the delivered diff matches the five origin/destination pairs and zero costs, preserves the existing resolver and other entries, and contains no unrelated code changes.
- [x] 1.2 Add `TestTeleportDestDungeon` coverage in `tmserver/internal/world/teleport_test.go` with independent expected coordinates for all 16 tiles of each origin block, zero cost, destination bounds and adjacent non-portals; verify the test cases cover both sides of the 147/148 boundary and preserve existing city regressions. Execution belongs to section 3.

## 2. Verify request-to-jump behavior

- [x] 2.1 Add handler tests named with the `TestReqTeleportDungeon` prefix using an in-memory world supporting the real coordinates and the established single-owner fixture pattern; verify coverage dispatches header-only 0x0290 for all five routes and compares decoded 0x036C Effect=1 destinations with authoritative position, with zero and positive gold unchanged. Execution belongs to section 3.
- [x] 2.2 Extend those handler regressions for dead/non-play sessions, unregistered origins, entry/return/re-entry and old/new view observers; verify the delivered assertions reject invalid travel without mutation and check normal visibility reconciliation without requiring exact random samples. Execution belongs to section 3.

## 3. Orchestrator validation in Docker

- [x] 3.1 After apply, have the orchestrator execute its automatic `make test`, `make build` and `make vet` checks in the Go image at `/workspace`; record results and address actual failures without weakening tests.
- [x] 3.2 After apply, have the orchestrator run `go test -race -count=20 -run 'TestTeleportDestDungeon|TestReqTeleportDungeon' ./tmserver/internal/world ./tmserver/internal/handler` from `/workspace`; verify both new test groups run and pass repeatedly. No compose services or environment variables are required.

Implementation and regression coverage are delivered. All six tasks are complete. On 2026-09-09, the orchestrator reported exit code 0 for make test (11:13:40 UTC), make build (11:14:46 UTC), make vet (11:15:28 UTC), and the focused race test with count=20 (11:16:02 UTC), all inside Docker. No tests, builds or code checks were executed on the host. The additional test command and empty service configuration are preserved for the publication rerun.
