## Purpose

Provide reliable, position-authoritative travel between Dungeon floors 1 and 2 for the unmodified WYD 7662 client.

## ADDED Requirements

### Requirement: Dungeon floor routes match legacy entrances and returns

The server SHALL resolve each of the following origin blocks to its destination base at zero gold cost. Each origin includes all 16 integer positions from the base through base plus 3 on each axis. Each destination SHALL add an offset in the inclusive range 0 through 2 independently on each axis.

| Origin base | Destination base | Direction |
|---|---|---|
| (144,3780) | (1004,4028) | Floor 1 to floor 2 |
| (148,3780) | (1004,4028) | Floor 1 to floor 2 |
| (1004,4028) | (148,3780) | Floor 2 to floor 1 |
| (408,4072) | (1004,4064) | Alternate floor 1 to floor 2 |
| (1004,4064) | (408,4072) | Alternate floor 2 to floor 1 |

#### Scenario: Enter through either adjacent block
- **WHEN** a living character in play requests teleport from any tile of (144,3780) or (148,3780)
- **THEN** the character arrives within X 1004..1006 and Y 4028..4030 without losing gold

#### Scenario: Return from the first landing
- **WHEN** a living character in play requests teleport from any tile of (1004,4028)
- **THEN** the character arrives within X 148..150 and Y 3780..3782 without losing gold

#### Scenario: Use the alternate passage in both directions
- **WHEN** a living character in play requests teleport from any tile of (408,4072) or (1004,4064)
- **THEN** the character arrives within base plus 0..2 on each axis of the corresponding destination in the table without losing gold

### Requirement: Requests preserve authoritative movement and client compatibility

The server SHALL use its current character position to resolve opcode 0x0290, including requests with no body, and SHALL retain existing in-play and liveness guards. Successful travel SHALL update authoritative position and send the existing 0x036C jump with Effect 1 and the corresponding destination, reconciling visibility through the normal teleport flow. These routes SHALL require no gold, items, guild membership, or event activation.

#### Scenario: Zero-gold character travels
- **WHEN** a living character in play with zero gold sends a header-only teleport request on a listed origin
- **THEN** its authoritative position and client jump destination agree within the specified landing area and gold remains zero

#### Scenario: Invalid character state
- **WHEN** a dead character or a session outside play requests one of these teleports
- **THEN** no teleport or gold mutation occurs

#### Scenario: Position outside a portal
- **WHEN** a character requests teleport from a nearby position outside all registered portal blocks
- **THEN** no teleport or gold mutation occurs

#### Scenario: Return and re-enter
- **WHEN** an eligible character travels to floor 2, explicitly requests the return, and explicitly requests entry again
- **THEN** each request follows the table without requiring a reconnect or server restart
