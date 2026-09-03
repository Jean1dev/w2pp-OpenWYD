## Why

Pedidos de Caça (items 3432 through 3437) are rejected as unknown consumables by the Go game server, so the unmodified WYD 7662 client cannot use their destination-selection interface. The complete legacy routing table and handling rules are available in `Source/Code/TMSrv/_MSG_UseItem.cpp`, allowing the missing behavior to be restored with protocol parity.

## What Changes

- Recognize `EF_VOLATILE` 195 as the Pedido de Caça consumable class.
- Support the six legacy scroll variants for Armia, Dungeon, Submundo, Kult, Kefra, and Nippleheim.
- Resolve the client's `WarpID` selection to the corresponding legacy destination, teleport the player, and consume one scroll unit.
- Preserve the item and player position when the item index or `WarpID` is outside the legacy ranges.
- Add automated coverage for destination routing, stack consumption, and invalid selections.

## Capabilities

### New Capabilities

- `hunting-scrolls`: Defines validation, destination selection, teleportation, and consumption behavior for Pedidos de Caça.

### Modified Capabilities

None.

## Impact

- Affects `_MSG_UseItem` handling in `tmserver/internal/handler` and its tests.
- Reuses the existing `MSG_UseItem.WarpID` decoder, authoritative world teleport path, item-stack handling, and structured logging.
- Introduces no protocol, persistence, database, client-data, or external dependency changes.
- World-state mutations remain inside the single-owner game loop.
