## 1. Equipment resource derivation and compatible saves

- [x] 1.1 Add runtime equipment-attribute HP/MP bookkeeping and player-only conversion in refreshScore before affects, preserving direct bonuses and percentages; verify delivered code computes 2 times resolved equipment CON/INT, overwrites deltas on refresh, and excludes base/affect attributes and non-players.
- [x] 1.2 Exclude the new runtime deltas from CharacterSave maxima while preserving the current persisted representation and deriveBaseScore contract; audit all snapshot construction and login/reset paths, verifying no RPC or database schema change and no stale delta after entity reuse or progression resets.

## 2. Regression coverage

- [x] 2.1 Add real-catalog tests for costumes 4185/4186 (+500 HP/MP), 4187 (+1000 MP only), and 4188 (+1000 HP only), plus resolved instance/refinement and other-equipment contributions; verify assertions cover resources and unchanged attribute/damage behavior across player classes/tiers.
- [x] 2.2 Add tests for mixed direct resource bonuses, percentage/Divine ordering, explicit CON affects, attribute allocation and level gains, no gear and non-player controls; verify the fixtures independently calculate expected maxima and detect double counting.
- [x] 2.3 Add handler transition tests for equip, replacement, removal, rejected moves and repeated refresh; verify score packet maxima, no healing on increases and clamps on decreases for injured/full resources.
- [x] 2.4 Add world snapshot and handler login regressions starting with a pre-fix saved costume, at least three save/login cycles, subsequent removal, expired costume on login and reset/reuse; verify stable persisted maxima, immediate correction of old saves and absence of leaked bonuses using in-memory persistence.

## 3. Documentation and orchestrated validation

- [x] 3.1 Update score/persistence comments and relevant migration documentation to explain the equipment-only 2x conversion, unchanged saved representation and intentional exclusion of legacy whole-maximum doubling; verify documentation matches implemented formulas and lifecycle tests.
- [x] 3.2 Have the orchestrator execute its automatic make test, make build and make vet inside Docker in /workspace and record results; verify all new regression tests are included and resolve failures without weakening assertions. No additional services or test commands are required.

Implementation, regression tests and documentation are delivered. On 2026-09-10 the orchestrator reported successful Docker execution in /workspace: make test (11:45:22 UTC), make build (11:46:55 UTC) and make vet (11:47:53 UTC), all exit code 0. No host code checks were run. Additional test commands and service requirements remain empty.
