## Why

Characters currently deal no damage to NPCs from their own kingdom, but the blocked attempt still places those NPCs and their groups into battle. Kingdom guard towers can consequently retain or attack allied players, contradicting the legacy clan-based alliance rules and issue #319.

## What Changes

- Treat Hekalotia clan 7 and Akelonia clan 8 as explicit same-kingdom alliances for player-to-NPC combat.
- Reject an allied kingdom attack before it can add enemies, select targets, drag an NPC group into battle, or apply aggressive skill effects.
- Require kingdom NPC AI, including guard towers, to discard allied players from existing battle state before attacking.
- Preserve hostile interactions between opposing kingdoms and the legacy hostility-table behavior for neutral and non-kingdom clans.
- Keep the guild-owned Tower War rules independent from kingdom guard-tower rules.

## Capabilities

### New Capabilities

- `kingdom-combat-relations`: Defines damage, battle-state, targeting, and guard-tower behavior between players and NPCs belonging to Hekalotia or Akelonia.

### Modified Capabilities

None.

## Impact

- Affects player attack resolution and aggressive-skill targeting in `tmserver/internal/handler/combat.go`.
- Affects NPC enemy validation, group battle propagation, and final mob attacks in `tmserver/internal/handler/mobai.go`.
- Uses the existing `Entity.Clan`, legacy `g_pClanTable`, and kingdom constants; no protocol, persistence, database, or client changes are expected.
- Requires focused regression tests covering both kingdoms, guard towers, opposing kingdoms, neutral clans, stale battle targets, and Tower War isolation.
