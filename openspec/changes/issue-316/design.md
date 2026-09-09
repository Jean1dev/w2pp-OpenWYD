## Context

See proposal.md for motivation. Initial inspection found a clean working tree and no existing issue-316 artifacts. The change uses the repository's spec-driven schema.

Evidence:
- `tmserver/internal/handler/misc.go:263-320`: `kingCapeService` uses `sendChatText` for already-complete, quote, and exact-payment feedback. Requirement and persistence failures use `notify`; success emits equipment/inventory/score updates.
- `tmserver/internal/handler/chat.go:411-414`: `sendChatText` explicitly sets `Header.ID` to `s.Conn`, explaining player attribution.
- `Source/Code/TMSrv/SendFunc.cpp:1779`: `SendSay` uses the mob identifier in chat. Its grid multicast is separate from speaker identity. `SendClientMessage` at line 27 instead emits a panel with ID zero; the legacy KING branch also uses panel messages. We do not claim every legacy cape reply was NPC chat.
- `docs/migration/handlers/_MSG_Quest-npcs.md`: NPC index and confirmation contract; `docs/migration/protocol-spec.md`: 12-byte header and chat/panel opcodes. The header table's generic player-ID description must be interpreted with the message-specific `SendSay` evidence.
- `openspec/specs/kingdom-cape-service/spec.md`: existing pricing, payment, transitions, and persistence contracts.
- `tmserver/internal/handler/quest_test.go`: packet harnesses, `questNPCTemplate`, `startServerQuestNPC`, and supported King forms; `kingdom_cape_test.go`: payment and target tests.

## Goals / Non-Goals

**Goals:** Separate recipient session from NPC speaker in these three replies while retaining the existing wire body encoding and single-owner loop.

**Non-Goals:** Global chat refactoring, NPC renaming, broadcast parity, conversion to panel messages, Arch changes, new purchase acknowledgement, economic or persistence fixes.

## Decisions

1. Add a small handler-local NPC reply helper accepting an explicit runtime NPC identifier, recipient session, and text. Send `MsgMessageChat` with that identifier and the same null-terminated body convention as the current helper. Replace only the three cape-service calls. Do not change `sendChatText` globally: its other callers rely on player attribution. Do not prefix NPC names in text: the client resolves the speaker through the header.
2. Preserve unicast via `World.SendTo`. Copying legacy `SendSay` multicast would expose individual purchase feedback to bystanders and expand this issue's behavior.
3. Capture the interacted NPC's runtime ID inside the owner loop before starting the quote operation, and carry that value into its completion callback. Use the actual entity index, never Merchant, Clan, generator index, a fixed King ID, or the connection ID. Keep blocking persistence under `World.Go`; sending still occurs in the loop callback. Preserve existing session-lifetime handling.
4. Reuse handler packet harnesses with fake persistence; add table-driven cases named under `TestKingCapeDialogue`. Cover quote, insufficient/exact-payment rejection, and complete mode for both clans, including canonical Merchant 111 and legacy 14/15. Ensure player and NPC IDs differ; assert decoded opcode, header ID, text, recipient isolation, and unchanged state/no purchase on these branches. Add a control for ordinary player-attributed replies and retain existing purchase/Arch tests. No external database is needed for this packet attribution change.

## Risks / Trade-offs

- Client rendering is inferred from packet identity and legacy `SendSay`; no client capture was executed or screenshot inspected. Mitigation: decoded packet regressions are the automated oracle; manually interact with both Kings on unmodified build 7662 when available to confirm displayed names.
- Async quote completion could accidentally revert to the session ID. Mitigation: test the asynchronous quote and rejection paths, not only the synchronous complete branch.
- Broad helper edits could affect unrelated dialogue. Mitigation: keep replacements scoped to `kingCapeService` and include the player-chat control.

## Migration Plan

Apply handler and regression-test changes together. The orchestrator runs `make test`, `make build`, and `make vet` in Docker; no host checks are authorized. Existing automatic tests cover the added cases, so no additional command or compose service is required. Deploy with the normal tmserver release; rollback restores the prior handler. No data migration is needed.
