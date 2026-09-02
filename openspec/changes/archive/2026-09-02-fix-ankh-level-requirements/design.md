## Context

See `proposal.md` for motivation. Equip validation reads the dot-separated requirements from `Release/Common/ItemList.csv`, while the unmodified WYD 7662 client renders its tooltip from the XOR-obfuscated, fixed-record `Release/DBsrv/run/ItemList.bin`. The client displays the stored level requirement one-based. Issue #278 changed the three Ankh raw requirements from 238/257/260 to 158/177/180 through a generic class-D rescale, but issue #308 requires one common displayed level of 161.

## Goals / Non-Goals

**Goals:**

- Make all three Ankh requirements equivalent to displayed level 161.
- Preserve byte-level agreement between the requirement fields in the CSV and compiled binary catalogs.
- Protect the item-specific values and cross-catalog parity with regression tests.

**Non-Goals:**

- Changing the general Mortal/Arch/Celestial equip-validation rules.
- Reworking the class-D rescaling policy from issue #278 for other items.
- Changing item effects, prices, slots, names, visuals, or attribute requirements.
- Introducing a new catalog compiler or client patch-delivery mechanism.

## Decisions

### Store raw requirement level 160 for all three Ankhs

Set the first requirement component for items 661, 662, and 663 to 160. The supported client renders this value as 161, matching issue #308, and the server already compares the same raw convention against character state.

Alternative considered: subtract another fixed amount from each post-#278 value. This would preserve three different requirements and would not satisfy the issue's common level-161 target.

### Patch both catalog representations atomically

Update the CSV source used by Go services and the corresponding requirement fields in `ItemList.bin` in the same change. The binary format remains 6500 XOR-0x5A records of 140 bytes, so only the existing `ReqLvl` field in records 661 through 663 changes.

Alternative considered: change only the server CSV. This would enforce the new level server-side while leaving the client tooltip stale, reproducing the reported mismatch.

### Extend existing catalog regression tests

Add the three items as explicit anchors in the real-catalog requirement test and the binary/CSV parity test. Reuse the independent parsers already present rather than adding production parsing code or a special case to `meetsEquipReq`.

Alternative considered: add a handler-only unit test with a synthetic requirement map. That would test generic comparison logic but would not detect stale content files, which are the source of this defect.

## Risks / Trade-offs

- [Clients keep an older `ItemList.bin`] -> Include distribution of the updated binary in release/deployment verification; server correctness alone cannot update the tooltip.
- [Manual binary editing changes adjacent bytes] -> Limit edits to decoded `ReqLvl` fields and run the full binary/CSV parity regression.
- [The issue intended only one Ankh] -> The issue uses the plural and states one desired level; treating all three consecutive Ankh items consistently is the recorded scope.

## Migration Plan

1. Update all three CSV `ReqLvl` values and their matching binary records in one commit.
2. Run focused content and item-catalog tests, then the repository test suite appropriate for the data-only change.
3. Deploy the server content and distribute the updated `ItemList.bin` to supported clients together.
4. Verify each tooltip displays level 161 and test equip attempts immediately below and at the threshold.

Rollback restores both catalog files together to prevent server/client disagreement.
