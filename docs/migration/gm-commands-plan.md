# GM / moderation in-game commands (issue #122)

Status: **IMPLEMENTED** (first batch).

Ports the legacy GM/moderation command surface (`Source/Comandos GM.txt`,
`Source/Code/TMSrv/imple.cpp` `ProcessImple`) to the Go rewrite as an authorized,
audited in-game command bus. Replaces the legacy authority model — a fragile
inflated character `Level >= 1000` plus a `DBSRV/Run/admin.txt` IP whitelist — with
an explicit server-side role.

## Authority

- Privilege = **`account.role`** (`'player'`/`'moderator'`/`'admin'`, migration
  `0005`, already used by the web-api for admin RPCs). It is now also returned at
  login so the game loop can gate commands.
- Flow: `store.AccountByName` (selects `role`) → `AccountLoginResponse.role`
  (`api/db/v1`) → `world.LoginOutcome.Role` → `world.Session.AccessLevel`
  (`world.ParseAccess`, fail-closed). Two GM tiers today: `moderator` runs the
  whole first batch; `admin` is reserved for future destructive/server-wide ops.
- The gate lives in `handler/gm.go` `runGMCommand`: `AccessLevel < AccessModerator`
  is silently denied (legacy behaviour) and logged. `kick` additionally refuses a
  target of equal-or-higher tier (legacy `imple.cpp:1824`).

## Entry point & syntax

`/gm <sub> <args>` — the client sends `/gm` as a whisper to target `"gm"` with the
rest of the line in the whisper `String`. Intercepted in `handler/chat.go`
`runCommand` (the same `_MSG_MessageWhisper` quirk the teleport commands use),
which now forwards the args string. Single entry point centralizes the authz gate
and the audit log, mirroring `ProcessImple`.

## First batch

`kick`, `notice`/`aviso`, `goto`/`ir`, `summon`/`puxar`, `spawn`, `item`,
`setlevel`, `setgold`, `ban`, `unban` (see `docs/game.md`). All world-state
mutations run **inside the single-owner loop** and reuse existing handler helpers
(`doTeleport`, `SpawnMobAt`+`revealSpawned`, `AddToCarry`, `applyMortalLevelUps`,
`sendEtc`, `Close`) — no new world mechanics. `notice` broadcasts via
`ForEachPlaying(-1)`. `spawn` uses the in-memory summon-template roster (the only
ID-indexed mob catalog held by the dispatcher).

## Ban

- `/gm ban`/`unban` write **`account.is_blocked`** through a new dbServer RPC
  `SetAccountBlocked` (off the loop via `World.Go`). Login already rejects blocked
  accounts (`LOGIN_RESULT_BLOCKED`), so a ban denies re-login immediately and, if
  the target is online, kicks it; unban restores.
- Argument resolves to the online player's account when present, else is taken as
  an account name (so an offline account can still be un/banned).
- **Deferred:** moving administrative ban to binServer entitlement
  (`web-platform-plan.md §binServer`, the `is_blocked` vs `StatusBlocked`
  source-of-truth question). Until then `is_blocked` is the single ban gate.

## Audit

Every command execution is logged with `slog` (account, id, role, subcommand,
args) before dispatch — the structured equivalent of the legacy `Log("adm ...")`.
No DB audit table this batch (unlike `npc_audit`); slog covers the acceptance
criterion.

## UNVERIFIED / follow-ups

- The dedicated golden **announce wire** (big centred notice) is not captured;
  `notice` ships as a `[GM]`-prefixed chat line for now (`notice.go §UNVERIFIED`).
- `setlevel` only levels **up** (reuses `applyMortalLevelUps`); a downlevel would
  need to unwind the derived score — out of scope.
- `weather` (`/gm weather <0|1|2|auto>`) shipped with issue #116 (`handler/weather.go`):
  it ports the legacy `/weather` (`imple.cpp:1122-1129`) but validates the value and adds
  the `auto` reset the original lacked.
- Later batches from `Comandos GM.txt` (events, guild/castle war, rates,
  mute/snoop) and the binServer ban migration are separate tasks.
