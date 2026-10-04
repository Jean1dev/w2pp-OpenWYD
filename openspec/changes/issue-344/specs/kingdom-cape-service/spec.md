## ADDED Requirements

### Requirement: King dialogue frames carry the full chat struct
Every `MsgMessageChat` (0x0333) frame the server sends, including the King's cape-service dialogue, SHALL carry the fixed-width `MSG_MessageChat.String` body of 96 bytes, NUL-terminated, matching the legacy `SendSay`. Speaker identity and private delivery from the cape dialogue requirement remain unchanged.

#### Scenario: Arch carrying the Pedra Ideal talks to the King
- **WHEN** an Arch of level 356 or more, carrying or equipping the Pedra Ideal, talks to either King with or without confirming
- **THEN** every chat reply carries the 96-byte body, the character is not sent back to character selection, and the session stays in play

#### Scenario: Over-long text
- **WHEN** a chat text reaches or exceeds 96 bytes
- **THEN** it is truncated to 95 bytes and the last byte stays NUL
