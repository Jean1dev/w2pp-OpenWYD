# Kingdom Teleport Commands Specification

## Purpose

Define safe, cape-aware chat commands for travel to a character's kingdom king or commerce area while preventing direct selection of an opposing kingdom.

## Requirements

### Requirement: King command aliases route by equipped cape
The server SHALL recognize `/rei` and `/king` as equivalent commands and SHALL determine their destination exclusively from the character's equipped cape.

#### Scenario: Blue kingdom cape travels to the blue king
- **WHEN** a character wearing a Hekalotia kingdom cape invokes `/rei` or `/king`
- **THEN** the server teleports the character to exactly `(1748,1574)`

#### Scenario: Red kingdom cape travels to the red king
- **WHEN** a character wearing an Akelonia kingdom cape invokes `/rei` or `/king`
- **THEN** the server teleports the character to exactly `(1748,1880)`

#### Scenario: Neutral cape cannot select a king
- **WHEN** a character with no equipped kingdom cape invokes `/rei` or `/king`
- **THEN** the server consumes the command without teleporting the character

### Requirement: Kingdom command aliases route to commerce or neutral center
The server SHALL recognize `/reino` and `/kingdom` as equivalent commands and SHALL route the character according to the kingdom represented by the equipped cape.

#### Scenario: Blue kingdom cape travels to blue commerce
- **WHEN** a character wearing a Hekalotia kingdom cape invokes `/reino` or `/kingdom`
- **THEN** the server teleports the character to exactly `(1690,1618)`

#### Scenario: Red kingdom cape travels to red commerce
- **WHEN** a character wearing an Akelonia kingdom cape invokes `/reino` or `/kingdom`
- **THEN** the server teleports the character to exactly `(1690,1842)`

#### Scenario: Neutral cape travels to the kingdom center
- **WHEN** a character with no cape, a white cape, a green cape, or another cape that represents neither kingdom invokes `/reino` or `/kingdom`
- **THEN** the server teleports the character to exactly `(1702,1726)`

### Requirement: Direct color teleport shortcuts are retired
The server MUST NOT recognize `/red` or `/blue` as teleport commands.

#### Scenario: Red shortcut is entered
- **WHEN** a character invokes `/red`
- **THEN** the server does not teleport the character

#### Scenario: Blue shortcut is entered
- **WHEN** a character invokes `/blue`
- **THEN** the server does not teleport the character

### Requirement: Kingdom command teleports preserve world consistency
Every successful kingdom command teleport SHALL use the server's authoritative teleport behavior so the character position and nearby entity visibility remain consistent within the single-owner world loop.

#### Scenario: Kingdom command teleport completes
- **WHEN** any recognized king or kingdom command selects a destination
- **THEN** the server updates the authoritative character position and emits the normal teleport and visibility updates
