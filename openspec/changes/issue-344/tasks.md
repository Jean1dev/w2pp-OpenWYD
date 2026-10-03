## 1. Reproduce

- [x] 1.1 Change `TestKingCapeDialogue` to require the fixed-width chat body and add `TestKingArchIdealStoneDialogue` (Arch 356/399, Pedra Ideal carried or equipped, cape complete or not, confirm 0/1, King merchant 14/15/111): every `MsgMessageChat` must carry a 96-byte body, no return to character selection, and the session must still answer `/cp`. Both failed before the fix (27-46 byte bodies).

## 2. Fix

- [x] 2.1 `EncodeMessageChatBody` keeps the last byte NUL; `sendChatText`, `sendNPCChatText`, `/nick`, and `gmNotice` use it.
- [x] 2.2 `docs/game.md` documents Celestial creation (right-click on the Pedra Ideal).

## 3. Verify

- [x] 3.1 `go test -race ./tmserver/...`, `go vet ./...`, and `go build ./...` (Docker).
- [ ] 3.2 Manual, after deploy: an Arch 356+ with the Pedra Ideal talks to both Kings on the unmodified 7662 client without disconnecting; right-clicking the stone creates the Celestial.
