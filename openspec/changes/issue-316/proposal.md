## Why

Issue #316 reports that King cape-service messages appear under the player's name. The Go handler sends those replies through player-attributed chat, so the client receives the player's identifier as the speaker.

## What Changes

- Attribute the three existing textual cape-service replies (price, exact-payment rejection, and already-complete cape) to the interacted King.
- Preserve private delivery to the requesting session, existing text, notices, prices, eligibility, payment, persistence, and Arch dispatch.
- Add packet-level regression coverage for both kingdoms and all supported King merchant shapes.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `kingdom-cape-service`: Require King dialogue to carry the interacted NPC's runtime speaker identifier.

## Impact

Changes are limited to tmserver handler reply construction and handler tests. No client patch, protocol opcode/layout change, database migration, external dependency, or service is required. This change contains planning artifacts only; implementation follows in apply.
