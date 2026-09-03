## Why

The normal Mortal-to-Arch-to-Celestial progression is broken across issues #305, #306, and #310: the Royal Cape quest cannot be entered, Kings do not provide the sapphire-based cape service, and using the Ideal Stone on a valid Arch can silently fail or create a character that diverges from legacy state. These paths should be restored together because they form one player-facing progression chain and share the King, cape, and tier-transition rules.

## What Changes

- Route and implement the `QUEST_CAPAREAL` NPC interaction so eligible Mortal characters can enter the Royal Cape quest area.
- Complete the King cape service, including cape-state classification, kingdom validation, dynamic sapphire pricing, sapphire consumption, cape assignment/upgrades, and kingdom population accounting.
- Bring Ideal Stone Arch-to-Celestial evolution into parity with the legacy 7640 server: eligibility, progression metadata, starting equipment, score/skill reset, celestial cape/body effects, persistence, and client refresh/logout behavior.
- Preserve the existing Mortal-to-Arch King flow and single-owner world-loop invariant while returning visible rejection feedback for unmet requirements.
- Add focused regression tests for both kingdoms, neutral capes, sapphire denominations and costs, quest bounds, every Ideal Stone level band, invalid prerequisites, persistence, and emitted protocol updates.

## Capabilities

### New Capabilities

- `royal-cape-quest`: Entry requirements and teleport behavior for the Royal Guard cape quest reported in #306.
- `kingdom-cape-service`: King interactions that assign or upgrade kingdom capes in exchange for the dynamically priced sapphire amount reported in #305.
- `celestial-evolution`: Legacy-compatible Arch-to-Celestial transformation through the Ideal Stone reported in #310.

### Modified Capabilities

None.

## Impact

- `tmserver/internal/handler/`: quest dispatch, King interaction, Ideal Stone use, score derivation, item consumption, persistence sequencing, and protocol notifications.
- `tmserver/internal/world/`, `internal/domain/`, `internal/store/`, and database migrations: additional legacy progression state required to reproduce and persist the transformation safely.
- Protocol-visible character equipment, score, effects, teleport, character-selection transition, and error feedback.
- Tests and migration documentation covering `_MSG_Quest.cpp` and `_MSG_UseItem.cpp` parity.
