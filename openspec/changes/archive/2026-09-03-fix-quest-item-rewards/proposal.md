## Why

Quest reward items with `EF_VOLATILE=191` currently fall through the Go item-use dispatcher, so using them grants neither their configured experience and gold nor the party experience share. The same item family is also absent from the stack policy, causing a second player-visible parity failure reported in issue #304.

## What Changes

- Support using all five legacy quest reward items, IDs `4117` through `4121`, rather than limiting the fix to the three items already tested by the reporter.
- Load their level ranges, experience rewards, and gold rewards from `Common/Settings/QuestsRate.txt`, preserving the legacy content configuration.
- Grant the user the full configured reward and grant 10% of its experience to each other connected member of the user's party.
- Apply normal experience caps, gold caps, level progression, client notifications, and item persistence/synchronization.
- Reject use outside the configured level range without consuming the item.
- Allow these quest items to merge and split using the existing maximum stack size, and consume exactly one unit after a successful use.

## Capabilities

### New Capabilities

- `quest-item-rewards`: Defines configuration, use, party distribution, consumption, and stacking behavior for the five `EF_VOLATILE=191` quest reward items.

### Modified Capabilities

None.

## Impact

- Affects quest-rate content loading and tmserver startup wiring.
- Affects the single-owner item-use and inventory merge/split paths in `tmserver/internal/handler`.
- Reuses the existing level-up, score/ETC synchronization, EXP notification, and persistence mechanisms; no wire-format or database-schema changes are expected.
- Requires regression coverage for all five item tiers, boundary levels, party roles, caps, rejected use, and stacked consumption.
