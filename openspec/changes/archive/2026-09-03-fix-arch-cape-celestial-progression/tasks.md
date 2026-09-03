## 1. Persistence Foundations

- [x] 1.1 Add typed `MortalLevel` and `CelestialArchLevel` fields across character domain, world, API/protobuf, fake stores, and live store mappings; verify generated code/build fixtures and round-trip unit tests preserve both values.
- [x] 1.2 Add a forward database migration for the progression fields with conservative defaults/backfill; verify migration integration tests load existing characters and save new values.
- [x] 1.3 Update Mortal-to-Arch creation to record the source Mortal progression level; verify Arch-creation handler and store tests assert the persisted value.
- [x] 1.4 Add persisted versioned kingdom sapphire-balance state and domain-specific quote/purchase persistence operations; verify concurrent store integration tests enforce one committed revision and the 4-32 bounds.

## 2. Royal Cape Quest (#306)

- [x] 2.1 Route Merchant 100, grade 13 to a dedicated Royal Cape quest handler without changing grade 4 Quest 256 routing; verify real-template and routing tests distinguish both Guards.
- [x] 2.2 Implement Mortal and stored-level `[199,254)` gates, visible rejection feedback, and legacy randomized teleport near `(1740,1725)`; verify boundary, class, coordinate-range, and RNG call-order tests pass.

## 3. Kingdom Cape Service (#305)

- [x] 3.1 Encode the legacy cape-to-kingdom and cape-mode classification plus tier-specific cape transitions as focused domain logic; verify table-driven tests cover every relevant cape index for both Kings and neutral state.
- [x] 3.2 Implement asynchronous King quote retrieval and client-visible 4-32 sapphire pricing while preserving the existing Mortal-to-Arch ask/confirm flow; verify handler tests cover balanced, minimum, maximum, and opposing prices.
- [x] 3.3 Implement exact sapphire payment planning for items 697 and 4131 without excess-value destruction; verify table-driven tests cover exact, mixed, insufficient, and non-representable denomination cases.
- [x] 3.4 Implement confirmed cape purchase as a staged, revision-checked persistence operation whose result is applied in the world loop; verify success emits inventory/equipment/score updates and failures leave live and persisted character state unchanged.
- [x] 3.5 Add regression tests for opposing-king rejection, already-completed capes, Hekalotia/Akelonia assignments, balance movement, and simultaneous purchases; verify targeted handler tests and race-enabled store tests pass.

## 4. Arch-to-Celestial Evolution (#310)

- [x] 4.1 Replace the level-399-only gate with legacy prerequisites (`ClassMaster`, stored level `>=355`, empty body armor, and `MortalLevel>=99`) and specific rejection feedback; verify boundary and item-preservation tests pass.
- [x] 4.2 Implement the five Arch achievement bands and items 3500/3501/3502; verify table-driven tests cover stored levels 355, 369, 370, 379, 380, 397, 398, 399, and above-cap defensive behavior.
- [x] 4.3 Build the complete Celestial starting snapshot from authoritative class bases, including zero-based level/experience, attributes, HP/MP, AC/damage, mastery/free points, learned-skill mask, special bonus 855, and fresh quest gates; verify per-class score and skill tests match legacy fixtures.
- [x] 4.4 Apply kingdom cape 3197/3198/3199 and body effects 98/106, consuming exactly one Ideal Stone; verify equipment/effect packet tests cover both kingdoms, neutral state, and stacked stones.
- [x] 4.5 Persist the staged Celestial snapshot before publishing it to the world and returning the player to character selection; verify success/relogin tests load the complete state and injected persistence failures preserve the original Arch and stone.

## 5. Integration and Documentation

- [x] 5.1 Run focused tests for quest, King, item, character persistence, and store packages, then run `make fmt`, `make test`, `make vet`, and `make lint`; record or resolve every failure attributable to this change.
- [x] 5.2 Update the migration documentation with the implemented #305/#306/#310 flows, intentional 4-32 sapphire clamp, exact-payment behavior, persisted fields, and cited legacy source locations; verify documentation no longer describes these branches as unimplemented.
- [x] 5.3 Exercise the full local progression path from Royal Cape quest through King cape purchase and Ideal Stone evolution using an unmodified 7662 client or an equivalent protocol-level end-to-end fixture; verify the resulting character survives relogin as a correctly equipped Celestial.
