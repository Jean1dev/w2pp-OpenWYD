## 1. Persistence and gameplay

- [x] 1.1 Add crystal stage across domain, protobuf, DBServer, tmserver, migration and login/save; verify mapping tests and PostgreSQL stage persistence/rollback tests.
- [x] 1.2 Implement sequential crystals with bonuses and persistence-first consumption; verify socket tests for ordering, stacks, rejection, failed saves and stage-by-stage relog.
- [x] 1.3 Enforce Arch EXP locks and preserve locked Fairy Dust; verify large-reward level gates, conventional EXP rejection and both dust boundaries.
- [x] 1.4 Support ordered late Lindy unlocks and refresh cape visuals; verify observer packets, relog, Fame, invalid recipes, repeated slots and save failure tests.
- [x] 1.5 Apply Cythera crystal penalty; verify socket evolution/relog for stages 0..4 and stored levels 355/380/399 alongside existing PR #314 tests.

## 2. Validation

- [x] 2.1 Run build, full race tests, vet, lint and Docker PostgreSQL integration tests; record outcomes and validate OpenSpec artifacts.
