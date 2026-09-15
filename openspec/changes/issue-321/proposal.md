## Why

Issue #321 reports that costumes increase attributes and damage but fail to increase HP/MP. The Go equipment recalculation aggregates INT/CON without converting their equipment contribution into resource maxima, including catalog costumes with 250 and 500 points.

## What Changes

- Convert equipment CON into two flat maximum HP per point and equipment INT into two flat maximum MP per point for players, before existing percentage and affect processing.
- Use the shared equipment calculation, so costumes and other equipment granting the same effects behave consistently; retain current catalog values and damage rules.
- Preserve current HP/MP on increases and clamp on decreases, publish the recalculated score through existing packets, and prevent bonus accumulation across refresh/save/login.
- Preserve compatibility with saves made before this correction by keeping the persisted maxima representation unchanged.

## Capabilities

### New Capabilities

- `equipment-attribute-resources`: Equipment-derived INT/CON resource maxima, lifecycle updates and persistence compatibility.

### Modified Capabilities

None. Existing item-equip requirements describe eligibility, not derived score.

## Impact

Expected changes during apply: `tmserver/internal/handler/item.go`, score/affect comments and regression tests, runtime score bookkeeping in `tmserver/internal/world/session.go`, and character snapshot construction in `tmserver/internal/world/world.go`. Add focused handler/world tests and document the derivation in migration documentation. No client patch, catalog rebalance, RPC/schema migration, external service or dependency is needed. This change contains planning only.
