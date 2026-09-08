## 1. Kingdom Relation Primitive

- [x] 1.1 Add an explicit same-kingdom relation for only clan pairs `(7,7)` and `(8,8)`, and verify table-driven tests reject mixed kingdoms and equal neutral/non-kingdom clans.

## 2. Player-to-NPC Combat Admission

- [x] 2.1 Gate physical player attacks against same-kingdom NPCs before hit resolution and battle propagation, and verify tests assert zero damage with unchanged NPC `Mode`, `Target`, `EnemyList`, and group state for both kingdoms.
- [x] 2.2 Apply the same early rejection to aggressive skill targets, and verify tests assert no damage, no aggressive affect/tick, no battle state, and no kingdom-specific regression for valid opposing targets.
- [x] 2.3 Verify opposing clan 7/8 attacks still resolve damage and provoke surviving NPCs under the ordinary rules, while equal non-kingdom clans retain their existing behavior.

## 3. NPC AI and Guard Towers

- [x] 3.1 Extend retained NPC target validation to reject same-kingdom players and use existing invalid-target cleanup, and verify tests cover both an active allied target and an allied entry remaining in `EnemyList`.
- [x] 3.2 Add the final pre-RNG NPC-strike guard for same-kingdom players, and verify a stale forced target cannot reduce player HP or advance the combat RNG stream.
- [x] 3.3 Add regression cases using clan 7 and clan 8 guard-tower entities, and verify each tower ignores its allied kingdom while acquiring and damaging an eligible opposing character.

## 4. Isolation and Verification

- [x] 4.1 Verify the guild-owned Tower War attack gate still depends on guild ownership independently of kingdom clan, including same-clan opposing guilds and different-clan owning guild members.
- [x] 4.2 Run focused handler/world tests for kingdom combat and Tower War, then run `make test` to verify the complete race-enabled suite passes.
