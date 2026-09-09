## Why

Issue #309 reports that travel to Dungeon floor 2 fails. Static inspection finds that the Go teleport table omits all five legacy routes connecting floors 1 and 2, so requests at those origins are silently ignored.

## What Changes

- Restore both adjacent entrance blocks, the alternate entrance, and their two return routes using legacy coordinates, zero cost, and destination spread.
- Cover every tile of each 4-by-4 origin block and verify the request-to-jump behavior, including rejection and unchanged gold.
- Keep this change limited to Dungeon floors 1 and 2; do not port other missing teleport routes, change guild taxation, refactor RNG, or modify the client.

## Capabilities

### New Capabilities

- `dungeon-floor-teleports`: Position-based travel between Dungeon floors 1 and 2 compatible with the 7662 client.

### Modified Capabilities

None.

## Impact

Implementation will affect `tmserver/internal/world/teleport.go`, its tests, and handler regression tests using the existing `reqTeleport` / `doTeleport` pipeline. No new packets, persistence, dependencies, external services, or content assets are required. This change contains planning artifacts only.
