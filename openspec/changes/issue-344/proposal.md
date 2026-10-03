## Why

Issue #344 reports that an Arch (level 356+) carrying the Pedra Ideal loses the connection when it talks to the King. For an Arch, the King always answers through the cape service, and since #316 those replies are `MsgMessageChat` frames attributed to the King's NPC ID with a variable-length body (text + NUL). Every NPC chat the client ever received from the legacy server (`SendSay`, `SendFunc.cpp:1779`) and from the Go mob AI (`sendMobChat`) carries the full fixed-width `MSG_MessageChat` struct (12 + 96 bytes), and the shared `BASE_CheckPacket` (`Basedef.cpp:7130`) pins `Size == sizeof(MSG_MessageChat)`.

## What Changes

- All server-sent `MsgMessageChat` frames use the fixed-width `MSG_MessageChat.String` body (`protocol.EncodeMessageChatBody`): the King's NPC dialogue, the player-attributed `sendChatText` notices, `/nick`, and the GM `notice`.
- `EncodeMessageChatBody` always leaves the last byte NUL, so over-long text stays a terminated C string (as `SendClientMessage` does).
- Celestial creation stays on right-clicking the Pedra Ideal (`_MSG_UseItem.cpp`); the King has no Celestial path in the legacy server. `docs/game.md` now documents the entry.

## Capabilities

### Modified Capabilities

- `kingdom-cape-service`: King dialogue frames carry the full fixed-width chat body.

## Impact

tmserver handler/protocol only. No opcode, database, or client patch change.
