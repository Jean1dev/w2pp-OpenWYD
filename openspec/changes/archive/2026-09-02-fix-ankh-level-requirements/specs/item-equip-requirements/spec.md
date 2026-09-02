## Purpose

Defines consistent, player-visible equipment requirements across the authoritative server catalog and the catalog consumed by the unmodified WYD 7662 client.

## ADDED Requirements

### Requirement: Ankhs require displayed level 161
The system SHALL define the required character level for Ankh da Justica (item 661), Ankh da Eternidade (item 662), and Ankh da Gloria (item 663) as level 161 in the WYD 7662 client's player-visible level numbering.

#### Scenario: Client displays an Ankh requirement
- **WHEN** a player inspects any of items 661, 662, or 663 using the supported client catalog
- **THEN** the client displays `Level necessario: 161`

#### Scenario: Mortal meets the Ankh requirement
- **WHEN** a Mortal character at displayed level 161 with all other applicable requirements satisfied attempts to equip any of items 661, 662, or 663
- **THEN** the server accepts the level requirement

#### Scenario: Mortal is below the Ankh requirement
- **WHEN** a Mortal character below displayed level 161 attempts to equip any of items 661, 662, or 663
- **THEN** the server rejects the equip operation as not meeting the item requirements

### Requirement: Server and client requirement catalogs remain consistent
The server-authoritative catalog and the compiled WYD 7662 client catalog MUST encode equivalent level and attribute requirements for each Ankh.

#### Scenario: Catalog parity is verified
- **WHEN** automated catalog consistency checks decode items 661, 662, and 663 from both catalog representations
- **THEN** their level, strength, intelligence, dexterity, and constitution requirements are identical
