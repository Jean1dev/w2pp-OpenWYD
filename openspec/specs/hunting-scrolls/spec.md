# Hunting Scrolls Specification

## Purpose

Defines how the game server validates and executes Pedido de Caça destination selections sent by the unmodified WYD 7662 client.

## Requirements

### Requirement: Legacy hunting scroll variants
The system SHALL recognize items 3432 through 3437 with `EF_VOLATILE` 195 as the Armia, Dungeon, Submundo, Kult, Kefra, and Nippleheim Pedidos de Caça, respectively.

#### Scenario: Supported scroll is used
- **WHEN** a playing, living character uses an inventory item whose index is between 3432 and 3437 and whose consumable class is 195
- **THEN** the system processes it as the corresponding Pedido de Caça variant

#### Scenario: Consumable class has an unsupported item index
- **WHEN** a character uses an item with consumable class 195 whose index is outside 3432 through 3437
- **THEN** the system does not teleport the character or consume the item

### Requirement: Destination selection
The system SHALL map each supported Pedido de Caça and client-selected `WarpID` from 1 through 10 to the corresponding destination coordinates defined by the legacy `HuntingScrolls` table.

#### Scenario: Valid destination is selected
- **WHEN** a character uses a supported Pedido de Caça with a `WarpID` from 1 through 10
- **THEN** the character is teleported to the legacy destination selected by the scroll variant and `WarpID`

#### Scenario: Destination selection is out of range
- **WHEN** a character uses a supported Pedido de Caça with `WarpID` zero or greater than 10
- **THEN** the system does not teleport the character or consume the item

### Requirement: Successful scroll consumption
The system SHALL consume exactly one unit of a Pedido de Caça only after its item index and destination selection have been validated.

#### Scenario: Single scroll is consumed
- **WHEN** a valid Pedido de Caça destination is executed from an item stack containing one unit
- **THEN** the inventory slot becomes empty and is synchronized with the client

#### Scenario: One scroll is consumed from a stack
- **WHEN** a valid Pedido de Caça destination is executed from an item stack containing multiple units
- **THEN** the stack decreases by exactly one unit and the updated slot is synchronized with the client

### Requirement: Authoritative teleport behavior
The system SHALL execute Pedido de Caça travel through the authoritative world teleport behavior so that position and entity visibility remain consistent for the player and nearby sessions.

#### Scenario: Hunting scroll teleport completes
- **WHEN** a valid Pedido de Caça destination is executed
- **THEN** the server updates the character's authoritative position and emits the normal teleport and visibility updates
