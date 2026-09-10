## Context

See proposal.md for motivation. Initial git status/diff were clean; no issue-321 artifacts existed. Planning uses the local spec-driven schema.

Evidence inspected:

- `Release/Common/ItemList.csv:5917-5923`: 4185/4186 have 250 STR/DEX/INT/CON; 4187 has 500 STR/INT; 4188 has 500 DEX/CON. Their position mask is 4096 (equipment slot 12). Values above 255 belong to catalog effects, not byte-sized instance values.
- `tmserver/internal/handler/item.go`: `equipBonus` already resolves catalog/instance/refinement effects; `refreshScore` adds `b.con`/`b.intel` to attributes but only `b.maxHP`/`b.maxMP` to maxima. `effectiveMaxHP/MP` apply percentages and buffs at read time. `sendScore` uses effective maxima.
- `Source/Code/Basedef.cpp:3152-3163`: equipment INT/CON deltas are multiplied by two. The same block also adds the entire preexisting maximum a second time. That whole-maximum doubling is not the Go model and is not part of this correction.
- `docs/migration/captura-wyd-levelup.md` section 3 and `handler/misc.go`: allocation already contributes two resources per INT/CON. `affect_score.go:231+` explicitly adds HP for CON buffs; converting total or effective attributes would double-count them.
- `handler/character.go:300+` derives bases from saved flat scores and refreshes. `world/world.go:373-374` snapshots live MaxHP/MP directly today. Simply adding the conversion to both refresh and inverse login derivation would cancel the correction on old saves; changing only refresh would accumulate it on every login.
- Consulted `AGENTS.md`, Go development guidelines, migration game rules, domain model, data formats, protocol score references, item/character handler contracts, project overview, dependency report and Basedef component analysis. Keep world state in the owning loop, preserve explicit wire layouts and RNG call order.

## Goals / Non-Goals

**Goals:** Correct the missing equipment-only conversion through the existing score pipeline and retain compatibility with the old persisted maxima representation.

**Non-Goals:** Costume-specific hardcoded grants, item rebalance, legacy whole-maximum doubling, new soul/buff formulas, base-stat recalculation from level, client changes, expiration-system redesign or database migrations. Existing incidental issues in recovery of attributes/direct bonuses when gear expires are outside this correction; the newly introduced resource term must not survive expiration.

## Decisions

### 1. Convert the existing equipment aggregate for players

During `refreshScore`, compute HP delta as `2 * int32(b.con)` and MP delta as `2 * int32(b.intel)`, gated to players, before affect calculation. Add them to the existing base-plus-direct-equipment maxima. Use consistent player eligibility for both deltas and bookkeeping; mobs/summons receive zero. Keep the existing resolved item/refinement policy, including mount exclusions. Never derive this from total CON/INT or effective affect attributes.

This follows the shared effect semantics and avoids an ID allowlist that would fail for equivalent costumes or other equipment. Do not copy the legacy block's doubling of base and direct resources: it changes characters without costumes and is unrelated private-server tuning. Document this distinction alongside the correction.

### 2. Track and exclude only the new term from saved maxima

Add two narrowly named, runtime-only int32 fields on Entity (for example EquipmentAttributeHP/MP), overwritten on every refresh, zero for non-players and reset on login initialization. Live MaxHP/MP include these deltas so existing affects, transforms, healing limits and score publication naturally use corrected values. Character snapshot construction writes `MaxHP - EquipmentAttributeHP` and `MaxMP - EquipmentAttributeMP`, leaving the old persisted flat representation intact. Preserve current HP/MP verbatim as the existing percent/buff save path already does.

Keep `deriveBaseScore` subtracting direct equipment resource bonuses only, because stored maxima deliberately exclude the new term. After login, refresh adds the current costume delta exactly once. Old saves therefore immediately gain the correction, new saves remain compatible, and expiration before refresh cannot retain the new term. Audit every CharacterSave construction path and entity reuse/reset path, including progression/reset flows, so cached values cannot be stale.

Alternative: changing persistence to equipment-free maxima or adding a versioned migration would address broader historical issues but materially expands scope. Alternative: subtracting the new delta on load assumes historical saves already contained it and cancels this fix. Alternative: applying it only at effectiveMax reads would leave affects and direct MaxHP consumers inconsistent.

### 3. Preserve transitions and existing score transport

Use current trading/equip, login and score refresh paths; no new packet. Existing refresh clamps resources to effective maxima and does not heal when maxima increase. Keep percent order and affect contributions; update the stale comment saying CON feeds nothing to explain equipment conversion versus affect deltas. Verify displayed score bytes through handler packet assertions, not a new serializer.

## Risks / Trade-offs

- Cached delta out of sync with a direct maximum reset → audit resource assignments in login, level-up, rebirth/reset and snapshots; test reset then refresh/save. Keep cache writes paired with flat-score recalculation.
- Incorrect 500-point fixtures → load the real catalog through existing content test helpers; never encode 500 in an instance effect byte. Include instance and refined items separately.
- Unintended damage or buff changes → preserve current calculations and assert existing STR/INT damage behavior and explicit CON/HP buffs, plus percent order.
- Catalog/refinement differences → expected 500/1000 resource deltas apply to unrefined listed costumes; refined expectations use the established resolved attribute values.
- Persistence previews still use the historical flat representation → this matches existing percentage/buff preview limitations; accurate in-world login/update packets are the acceptance boundary.

## Migration Plan

No schema/data migration or services. Implement runtime bookkeeping, conversion and snapshot exclusion together. Run focused regression and existing automatic Go checks only in the orchestrator's Docker image. Deploy through the orchestrator's normal process. Reverting the cohesive code change restores prior behavior because persisted maxima retain their previous meaning.

## Validation Plan

Add handler tests for real 4185-4188 catalog items, all player classes/tiers, mixed direct and attribute bonuses, percentages/Divine, explicit CON affects, no-gear and non-player controls, equip/swap/unequip packets, rejected moves, repeated refresh, injured/full resources, and allocation/level gains remaining single-counted. Add snapshot/login regressions beginning with pre-fix saved values, at least three save/login cycles, removal and login expiration; assert both runtime and persisted values. Use in-memory persistence mocks and world test harnesses. No external services required. `make test`, `make build` and `make vet` are supplied by the orchestrator; no extra command beyond that suite is necessary. No code tests or builds were run during planning.
