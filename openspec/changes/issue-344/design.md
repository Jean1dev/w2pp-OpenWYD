## Context

Evidence:
- `tmserver/internal/handler/misc.go` `kingQuest`: only a Mortal 299+ with 1742/1760-1763 equipped reaches `kingArch`; an Arch always reaches `kingCapeService`, whose text replies go through `sendNPCChatText`.
- `tmserver/internal/handler/chat.go` `sendNPCChatText` (added by #316 on 2026-09-09, nine days before the issue): `MsgMessageChat`, `HEADER.ID` = NPC index, body = text + NUL. #316's design lists "no client capture was executed" as a risk.
- Legacy `SendSay` and Go `sendMobChat` send `sizeof(MSG_MessageChat)` for NPC speakers; this is the only short `0x0333` frame with an NPC speaker the client has ever received.
- Railway logs for the 2026-09-17 deployment that served the report had expired, so no `session last sends` post-mortem is available. The packet-shape divergence is the strongest evidence, not a capture.

## Decisions

1. Fix the encoder at the source: every `MsgMessageChat` sender uses `EncodeMessageChatBody`, not only the King. Keeping some short frames would leave the same latent divergence for player-attributed notices.
2. Keep #316's speaker identity and unicast delivery unchanged.
3. Do not add Celestial creation to the King: the legacy entry is the Pedra Ideal self-use, already implemented in `useIdealStone`.

## Risks

- Root cause not confirmed by a client capture. Mitigation: after deploy, repeat the reported scenario with both Kings; if it still disconnects, pull `session last sends` from the tm-server logs at the disconnect time (emitted on every disconnect) and follow the tail.
- Text longer than 95 bytes is now truncated. Current server texts are shorter; the legacy field is 96 bytes anyway.
