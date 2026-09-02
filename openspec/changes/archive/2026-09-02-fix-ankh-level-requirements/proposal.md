## Why

The WYD 7662 client reports that Ankhs require level 258, while issue #308 establishes level 161 as the intended requirement. The existing class-D rescale from issue #278 reduced these three items inconsistently and can still leave the client catalog showing stale requirements.

## What Changes

- Normalize the equip requirement of Ankh da Justica (661), Ankh da Eternidade (662), and Ankh da Gloria (663) so the client displays required level 161.
- Keep the server CSV catalog and the compiled client `ItemList.bin` representation synchronized for these items.
- Add regression coverage for the three raw requirements and CSV/binary parity.

## Capabilities

### New Capabilities

- `item-equip-requirements`: Defines authoritative equip-level requirements and server/client catalog parity for equippable items, initially covering the three Ankhs from issue #308.

### Modified Capabilities

None.

## Impact

- Content data: `Release/Common/ItemList.csv` and `Release/DBsrv/run/ItemList.bin`.
- Catalog and equip-requirement regression tests under `tmserver/internal/content` and `webserver/internal/itemcatalog`.
- No protocol, database schema, API, or game-loop ownership changes.
- Deployment must distribute the updated `ItemList.bin` to WYD 7662 clients for the displayed requirement to match server enforcement.
