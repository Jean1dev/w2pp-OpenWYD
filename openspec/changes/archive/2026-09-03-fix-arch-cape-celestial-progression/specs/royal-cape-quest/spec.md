## Purpose

Restore the Royal Guard entry point that lets eligible Mortal characters reach the Royal Cape quest encounter used by the normal kingdom progression.

## ADDED Requirements

### Requirement: Eligible characters can enter the Royal Cape quest
The server SHALL recognize the Royal Cape quest NPC independently from the similarly named Quest 256 Guard and SHALL teleport a Mortal whose stored level is at least 199 and below 254 to the Royal Cape quest area near coordinates `(1740,1725)`.

#### Scenario: Eligible Mortal uses the Royal Guard
- **WHEN** a Mortal with stored level in `[199,254)` interacts with a Merchant 100, grade 13 Royal Guard
- **THEN** the server teleports the character to a legacy-compatible randomized position near `(1740,1725)`

#### Scenario: Arch cannot enter the Mortal quest
- **WHEN** a non-Mortal interacts with the Royal Guard
- **THEN** the server leaves the character in place and sends visible requirement feedback

#### Scenario: Character is outside the quest level range
- **WHEN** a Mortal below stored level 199 or at or above stored level 254 interacts with the Royal Guard
- **THEN** the server leaves the character in place and sends visible requirement feedback

### Requirement: Royal Guard routing does not collide with Quest 256
The server MUST route the Royal Cape quest using the NPC's legacy Merchant and grade identity and MUST preserve the separate Quest 256 Guard behavior.

#### Scenario: Quest 256 Guard remains operational
- **WHEN** an eligible character interacts with a Merchant 100, grade 4 Guard while carrying the required emblem
- **THEN** the server executes the existing Quest 256 final-stage behavior rather than the Royal Cape teleport

