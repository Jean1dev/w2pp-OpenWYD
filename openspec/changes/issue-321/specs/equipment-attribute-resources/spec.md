## Purpose

Ensure equipment-granted intelligence and constitution consistently contribute to player resource maxima throughout equipment changes and character sessions.

## ADDED Requirements

### Requirement: Equipment attributes contribute to resource maxima
The server SHALL add two flat maximum HP per equipment CON and two flat maximum MP per equipment INT for players, using the same resolved equipment attributes displayed in their score. These contributions SHALL precede existing resource percentage and affect processing. Base attributes and affect-granted attributes MUST NOT be counted again as equipment attributes. STR and DEX SHALL retain their existing effects without an additional HP/MP conversion.

#### Scenario: Costumes granting 250 attributes
- **WHEN** a player equips unrefined costume 4185 or 4186 without other resource modifiers
- **THEN** maximum HP and MP each increase by 500, and the existing attribute and damage bonuses remain

#### Scenario: Costumes granting 500 attributes
- **WHEN** a player equips unrefined costume 4187 without other resource modifiers
- **THEN** maximum MP increases by 1000 and maximum HP does not increase from attribute conversion
- **AND** equipping costume 4188 instead grants 1000 maximum HP and no attribute-derived MP

#### Scenario: Shared equipment effects
- **WHEN** another valid equipped item grants resolved INT or CON, including catalog, instance and refinement effects
- **THEN** each resolved point contributes once to the corresponding resource maximum using the same rule as costumes

#### Scenario: Percentages and affects
- **WHEN** a costume grants 250 CON and 250 INT to a player whose flat maxima without it are 1000 HP and 1000 MP, with 10 percent HP/MP equipment bonuses and Divine active
- **THEN** each effective maximum is 1980 using the existing percentage order
- **AND** explicit HP/MP bonuses from other affects are applied once under their existing rules

### Requirement: Equipment transitions update live resources without healing
The server SHALL recalculate and communicate the corrected maxima through the existing supported client score messages after accepted equipment changes. Current HP/MP SHALL remain unchanged on maximum increases and SHALL be clamped when they exceed the new effective maxima. Rejected changes SHALL leave equipment and score unchanged. Repeated recalculation SHALL be idempotent.

#### Scenario: Equip and unequip while injured
- **WHEN** an injured player equips a costume and later removes it
- **THEN** equipping does not heal HP or MP and removing it clamps only resources above the reduced maxima
- **AND** the client receives score values matching the server's effective maxima

#### Scenario: Replacement and repeated recalculation
- **WHEN** a player replaces a 250-attribute costume with a 500-attribute costume and score is recalculated repeatedly
- **THEN** maxima reflect only the currently equipped items without stacking prior contributions

#### Scenario: Invalid equipment move
- **WHEN** the server rejects an attempted costume equip or removal
- **THEN** neither resource maxima nor current resources change

### Requirement: Save and login preserve the correction
The server SHALL grant the correction to existing saved characters and retain stable equipment-free maxima across subsequent saves and logins, without requiring a database migration. Equipment removed before character entry SHALL not retain the new attribute-derived resource contribution. Non-player template resources SHALL retain their current behavior.

#### Scenario: Existing character already wearing a costume
- **WHEN** a character saved before the correction logs in wearing costume 4185
- **THEN** flat maxima gain 500 HP and 500 MP over the old behavior
- **AND** repeated save/login cycles preserve these maxima and unequipping restores the equipment-free maxima

#### Scenario: Costume expires before login
- **WHEN** a previously equipped costume is discarded as expired during login
- **THEN** the new attribute-derived HP/MP contribution from that costume is absent

#### Scenario: Non-player regression
- **WHEN** a mob or summon score is refreshed
- **THEN** this player equipment correction does not change its template resource maxima
