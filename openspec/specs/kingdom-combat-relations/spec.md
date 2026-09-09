# Kingdom Combat Relations Specification

## Purpose

Define legacy-compatible combat alliances between kingdom characters and kingdom NPCs so allied defenders never harm or engage their own citizens while opposing kingdoms remain hostile.

## Requirements

### Requirement: Same-kingdom player attacks are rejected without engagement
The server SHALL treat a player and a non-player entity as same-kingdom allies when both have clan 7 or both have clan 8. A physical attack or aggressive skill directed at such an ally MUST cause no damage, apply no aggressive effect, and create no battle relationship for the target or its group.

#### Scenario: Hekalotia character attacks a Hekalotia NPC
- **WHEN** a clan 7 character directs a physical attack or aggressive skill at a living clan 7 NPC
- **THEN** the NPC takes no damage or aggressive effect and neither the NPC nor its group enters battle with the character

#### Scenario: Akelonia character attacks an Akelonia NPC
- **WHEN** a clan 8 character directs a physical attack or aggressive skill at a living clan 8 NPC
- **THEN** the NPC takes no damage or aggressive effect and neither the NPC nor its group enters battle with the character

### Requirement: Opposing kingdom combat remains hostile
The server SHALL allow clan 7 and clan 8 entities to acquire, engage, and damage each other subject to the ordinary combat rules.

#### Scenario: Akelonia character attacks a Hekalotia NPC
- **WHEN** a clan 8 character validly attacks a clan 7 NPC
- **THEN** the attack resolves under the ordinary combat rules and the surviving NPC may enter battle with the character

#### Scenario: Kingdom NPC detects an opposing character
- **WHEN** a clan 7 or clan 8 NPC detects a living, attackable character from the opposing kingdom
- **THEN** the NPC may acquire and attack that character according to its ordinary AI range, leash, and cadence rules

### Requirement: Kingdom NPCs discard allied battle targets
A kingdom NPC MUST validate the current kingdom relationship before pursuing or damaging a player. If a previously recorded target is now a same-kingdom ally, the NPC SHALL remove that target from its battle state and SHALL NOT attack it.

#### Scenario: Existing target becomes an ally
- **WHEN** a clan 7 or clan 8 NPC has a player recorded as an enemy and that player is now in the same kingdom as the NPC
- **THEN** the NPC drops the allied target without moving toward it or causing damage

#### Scenario: Allied target remains in an enemy-list slot
- **WHEN** a kingdom NPC processes an enemy-list entry that refers to a same-kingdom player
- **THEN** the entry is not considered a valid combat target and cannot produce an attack

### Requirement: Kingdom guard towers protect only their opposing kingdom
Guard towers assigned to clan 7 or clan 8 SHALL use the same kingdom relationship rules as other kingdom NPCs. A guard tower MUST NOT acquire, pursue, or damage a player from its own kingdom and SHALL remain able to attack an eligible player from the opposing kingdom.

#### Scenario: Hekalotia tower sees a Hekalotia character
- **WHEN** a clan 7 guard tower evaluates a clan 7 character
- **THEN** the tower does not acquire, pursue, or damage the character

#### Scenario: Akelonia tower sees an Akelonia character
- **WHEN** a clan 8 guard tower evaluates a clan 8 character
- **THEN** the tower does not acquire, pursue, or damage the character

#### Scenario: Guard tower sees an opposing character
- **WHEN** a kingdom guard tower evaluates an otherwise attackable character from the opposing kingdom
- **THEN** the tower may acquire, pursue, and damage that character under the ordinary NPC combat rules

### Requirement: Non-kingdom and Tower War relations remain independent
The same-kingdom protection MUST apply only to the explicit clan 7 and clan 8 equal-clan pairs. Neutral and other clans SHALL continue to use the legacy clan-hostility matrix, and the guild-owned Tower War target rules SHALL continue to use guild ownership rather than kingdom affiliation.

#### Scenario: Equal non-kingdom clans interact
- **WHEN** a player and NPC have equal clan values other than 7 or 8
- **THEN** the server determines hostility using the existing non-kingdom rules rather than granting kingdom protection

#### Scenario: Guild-owned Tower War tower is evaluated
- **WHEN** a player attempts to attack the guild-owned Tower War tower during its event
- **THEN** eligibility is determined by the player's guild and the tower's owning guild, independent of clan 7 or clan 8
