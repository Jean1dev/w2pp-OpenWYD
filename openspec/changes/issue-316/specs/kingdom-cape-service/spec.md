## ADDED Requirements

### Requirement: Cape dialogue identifies the interacted King
The server SHALL send existing cape price, exact-payment rejection, and already-complete dialogue with the interacted King's runtime entity identifier as the speaker of `MsgMessageChat` (0x0333), never the requesting player's identifier. This MUST apply to both kingdoms and supported King merchant forms 14, 15, and 111. Delivery SHALL remain private to the requesting session, with existing text and cape-service state transitions preserved.

#### Scenario: Player requests a quote
- **WHEN** an eligible player asks either King for a cape price without confirming
- **THEN** the reply identifies that King and includes the current price, without charging or changing cape or balance state

#### Scenario: Exact payment is unavailable
- **WHEN** a player confirms but cannot provide the required exact sapphire payment
- **THEN** the rejection identifies the interacted King and preserves inventory, cape, and balance

#### Scenario: Cape is already complete
- **WHEN** a player presents a completed cape of the interacted King's kingdom
- **THEN** the acknowledgement identifies that King without charging or altering balance

#### Scenario: Another player is nearby
- **WHEN** the requesting player receives one of these replies while another player is nearby
- **THEN** only the requesting player receives the reply and its speaker remains the interacted King

#### Scenario: Other messages retain their semantics
- **WHEN** ordinary player-attributed chat, a requirement notice, or a successful cape purchase is emitted
- **THEN** its existing attribution, delivery, and state effects remain unchanged
