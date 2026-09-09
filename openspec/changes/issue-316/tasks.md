## 1. Correct cape dialogue attribution

- [x] 1.1 Add a handler-local NPC chat reply helper and replace the three `kingCapeService` text replies, capturing the runtime NPC ID before asynchronous quote work. Verify by reviewing the diff that opcode/body/text and recipient remain unchanged, `sendChatText` and notices retain their semantics, and all sends remain inside the owner loop.

## 2. Add regression coverage

- [x] 2.1 Add `TestKingCapeDialogue` packet cases for quote, exact-payment rejection, and already-complete replies across both clans and supported Merchant 14/15/111 shapes using fake persistence. Verify the delivered tests assert NPC header ID distinct from player ID, expected text, no purchase, and unchanged inventory/cape/balance for those branches.
- [x] 2.2 Add recipient-isolation and ordinary player-chat controls using the existing harness. Verify the tests assert no cape dialogue reaches a nearby second session and ordinary chat still carries the player's ID; preserve all existing assertions and purchase/Arch tests.

## 3. Orchestrator verification

- [x] 3.1 Obtain Docker results for the orchestrator's automatic `make test`, `make build`, and `make vet` in `/workspace`; verify all succeed and record results. No host execution, additional test commands, or compose services are required.

Implementation and 16 packet regression cases are delivered in the apply phase. No tests, builds, or code checks were executed on the host; task 3 is complete based on the successful Docker results returned by the orchestrator. Optional client follow-up: inspect the displayed speaker for all three replies with both Kings on unmodified build 7662.

Repair round 1: the orchestrator reported successful `make build` and `make vet`, but all 16 dialogue cases received requirement notices. The King fixture inherited a misplaced clan byte from the generic quest template helper (CurrentScore.Damage rather than STRUCT_MOB byte 16). The issue-specific fixture now writes the documented clan offset and asserts the parsed clan and merchant before starting the server. All dialogue, recipient-isolation, player-chat, and saved-state assertions remain intact. After this repair, the orchestrator reported all three Docker commands passing (exit 0) on 2026-09-09: make test at 11:40:37.411Z (test-1788953827080-0.log), make build at 11:41:32.890Z (test-1788954037525-1.log), and make vet at 11:42:16.991Z (test-1788954092963-2.log). Logs are under .loop/logs/issue-316 in the orchestrator workspace.
