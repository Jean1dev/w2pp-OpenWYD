## Context

See `proposal.md` for motivation and `specs/kingdom-combat-relations/spec.md` for the behavioral contract. The legacy attack handler identifies equal clan 7 or equal clan 8 player-to-NPC pairs, zeros damage, and exits that target entry before later battle propagation. The Go attack pipeline already suppresses same-kingdom damage, but its shared post-hit path deliberately provokes surviving mobs even after blocked hits.

Kingdom NPC templates already carry the required affiliation in `STRUCT_MOB.Clan`: `Torre_Guardia` is clan 7 and `Torre_Guardia_` is clan 8. NPC acquisition uses the legacy clan hostility table, while retained battle targets flow through a separate validation and attack path. Guild Tower War uses `Guild` ownership and must not be coupled to these clan rules. All changes execute inside the single-owner world loop.

## Goals / Non-Goals

**Goals:**

- Represent the legacy equal-kingdom rule once and apply it consistently at player attack admission, NPC target validation, and the final NPC strike boundary.
- Prevent rejected allied attacks from mutating NPC or group battle state.
- Remove stale allied targets safely without changing target selection or RNG order for valid enemies.
- Preserve current Tower War guild ownership behavior.

**Non-Goals:**

- Redesigning kingdom membership, cape purchase, or character persistence.
- Introducing a generic alliance or faction system beyond the legacy clan 7/8 rule.
- Changing the clan-hostility table, NPC template files, kingdom wall damage, king deaths, or RvR scheduling.
- Correcting moderator-created template overrides whose clan is intentionally configured differently.

## Decisions

### Use an explicit same-kingdom predicate

Introduce a narrowly scoped predicate that returns true only for `(7,7)` and `(8,8)`. Use it rather than equality alone or inversion of the clan hostility table: other equal clans have distinct gameplay meanings, and `g_pClanTable` is the ambient AI acquisition policy rather than the legacy player-attack `isFrag` rule.

Alternative considered: treat every equal clan as allied. Rejected because it broadens protection beyond the two conditions present in `_MSG_Attack.cpp` and can change monster, summon, and event behavior.

### Reject allied target entries before combat side effects

In the per-target player attack pipeline, apply the predicate before damage/effect resolution and before any call that populates `EnemyList`, selects `Target`, or propagates battle to `PartyList`. The rejected packet entry should carry the same no-hit result as other invalid targets without consuming combat RNG for that entry.

Alternative considered: leave damage resolution intact and only suppress `setGroupBattle`. Rejected because aggressive skills could still apply affects or trigger on-hit side effects before the late suppression.

### Revalidate retained player targets in NPC AI

Extend NPC target validity so a clan 7/8 NPC rejects a same-kingdom player. Existing invalid-target cleanup should then remove the target and allow the NPC to return to idle or select a valid remaining enemy. Keep the existing hostility-table check for NPC-versus-NPC and ambient acquisition.

Alternative considered: rely only on `FindEnemyFromView`. Rejected because it governs initial acquisition, not targets inserted by retaliation/group propagation or retained in `EnemyList`.

### Add a final guard before NPC damage

Recheck the same-kingdom relationship at the NPC strike boundary. This is a defense against stale state within an already scheduled combat cycle and guarantees towers cannot damage allies even if another path populates their target list. Perform the guard before combat RNG so rejected allied strikes do not perturb the parity-sensitive RNG stream.

Alternative considered: rely exclusively on target validation. Rejected because the reported friendly-fire consequence merits a cheap invariant at the point where HP would be mutated.

### Do not special-case tower names or generator indices

Kingdom guard towers inherit the common NPC rule from their template clan. Do not key behavior on `Torre_Guardia`, generator indices, coordinates, or equipment. This covers every kingdom defender that uses clan 7/8 and avoids confusing kingdom towers with the separate guild Tower War generator.

Alternative considered: patch only known guard-tower generators. Rejected because it would leave ordinary kingdom NPCs inconsistent and make content naming part of combat semantics.

## Risks / Trade-offs

- [An attack path bypasses the main per-target gate] -> Cover physical attacks, aggressive skills, retained AI targets, and final NPC strikes with focused tests.
- [A broad helper changes non-kingdom behavior] -> Pin the predicate to the exact `(7,7)` and `(8,8)` pairs and test equal neutral/other clans.
- [The final strike guard changes RNG parity] -> Execute it before calling hit resolution so rejected allied attacks consume no combat RNG.
- [Tower War accidentally inherits kingdom protection] -> Keep its generator/guild eligibility gate unchanged and add an isolation regression test.
- [A database mob-stat override sets a tower clan to zero] -> Treat the effective runtime clan as authoritative; configuration validation is outside this change.

## Migration Plan

No data or protocol migration is required. Deploy the tmServer change normally; existing characters and NPC templates already persist their clan values. Rollback consists of reverting the combat/AI change, with no stored-state conversion.
