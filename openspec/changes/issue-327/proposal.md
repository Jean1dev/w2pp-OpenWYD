## Why

Arch characters can bypass the 355/370 progression locks, the crystal quest is missing, and the Ideal Stone path does not apply the legacy incomplete-quest penalty. This breaks the intended Arch-to-Celestial progression, especially when experience comes from Fairy Dust.

## What Changes

- Add the four-step Arch crystal quest with persistent progress and legacy stat/EXP effects.
- Enforce Arch progression locks for combat, direct EXP, and Fairy Dust; preserve Fairy Dust while locked.
- Complete the Lindy unlock flow, including late recovery and kingdom cape visual updates.
- Apply Silver Cythera +0 for incomplete crystal quest and the existing banded reward for completed quest.
- Add regression coverage for persistence, boundaries, item consumption, and failed saves.

## Capabilities

### New Capabilities
- `arch-crystal-quest`: Sequential crystal quest progress, rewards, and persistence.

### Modified Capabilities
- `celestial-evolution`: Crystal completion controls the Celestial equipment outcome; Arch experience gates and Lindy unlock behavior are enforced.

## Impact

Affected tmserver handlers, world/entity persistence snapshots, DB protobuf mappings, migrations, existing celestial specifications, and integration tests.
