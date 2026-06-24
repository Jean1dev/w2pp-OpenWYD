# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this project is

A **big-bang rewrite in Go** of the WYD (With Your Destiny) MMORPG server, targeting the
**unmodified `WYD.exe` 7662 client** (protocol version **7640** — 7662 is the build/patch name).
The legacy C++ server sources live in `Source/`, the runnable legacy binaries + game content in
`Release/`, and the new Go services in `tmserver/`, `dbserver/`, `binserver/`. The rewrite is
driven by reverse-engineering documentation under `docs/migration/` — read it before changing
wire/format/gameplay behavior; it cites evidence as `Source/.../file.cpp:line` and marks unconfirmed
points **UNVERIFIED**.

Module path: `github.com/jeanluca/w2pp-openwyd` (Go 1.25/1.26, Linux/Docker target).

## Commands

```bash
make build          # go build ./...
make binaries       # build each service into bin/ (tmserver, dbserver, binserver)
make test           # go test -race -cover ./...
make lint           # golangci-lint run (needs golangci-lint v2; staticcheck/govet/errcheck/gosec)
make vet            # go vet ./...
make fmt            # gofmt -w . + goimports
make vuln           # govulncheck ./...
make proto          # regen gRPC from api/*.proto (needs protoc + protoc-gen-go[-grpc])
make certs          # generate dev mTLS certs into ./certs (gitignored)
```

Run a single test / package:
```bash
go test -run TestChecksum ./tmserver/internal/protocol
go test -race ./tmserver/internal/world
go test -tags=integration ./dbserver/...   # integration tests are behind the `integration` build tag
```

Running the stack:
```bash
make run            # tmserver only, no-op persistence (no -dbserver) — quick protocol bring-up
make run-local      # full stack via docker compose + seed test account, prints client IP:port
docker compose up --build                                   # full topology, insecure internal links
docker compose -f docker-compose.yaml -f docker-compose.mtls.yaml up --build   # with mTLS (run `make certs` first)
```

`make run-local` (`scripts/run-local.sh`) seeds account `test`/`test123` (override with
`W2PP_LOCAL_ACCOUNT`/`W2PP_LOCAL_PASSWORD`) and is the path for pointing a real Windows client at the
server. The account has no characters — create them in the client.

## Architecture

Three microservices (`migration-plan.md §3.5`). Only the client↔tmServer edge speaks the legacy
protocol; internal links are gRPC (+mTLS):

- **tmServer** (`tmserver/`, port `8281` game + `80` status) — the game server. Speaks the legacy
  **CPSock** wire protocol to the client and owns all in-memory world state. Channel-status page
  (`serv00.htm`) is served over plain HTTP on `:80`, separate from the game port, because the client
  probes status there before opening the CPSock connection.
- **dbServer** (`dbserver/`, port `7514`) — persistence over gRPC (`api/db/v1`) backed by
  PostgreSQL (pgx v5). Subcommands: `serve`, `convert` (one-shot legacy account-file → DB import),
  `seed-account` (idempotent password-only account for local testing).
- **binServer** (`binserver/`, port `3000`) — billing gate over gRPC (`api/bin/v1`).

Wiring is in each `cmd/<svc>/main.go` (flags, logging, gRPC clients, listeners, graceful shutdown
via signal-cancelled context). tmServer degrades gracefully: without `-dbserver` it uses no-op
persistence (logins report no account); without `-binserver` billing is allow-all.

### The single-owner game loop (the one invariant that matters)

All world state is owned by **exactly one goroutine** (`world.World.Run`) and is never mutated
elsewhere — there are **no locks on world state**. This mirrors the original single-threaded WinSock
reactor and is what preserves gameplay parity and prevents item duplication. Network I/O runs in
per-connection goroutines that only exchange messages with the loop over channels (events in,
per-session out). See `tmserver/internal/world/world.go` package doc.

Consequences for any code you add to the game path:
- **Handlers run inside the loop goroutine** (`tmserver/internal/handler/`), so they may mutate world
  state directly without synchronization.
- **Blocking calls (dbServer/billing) must NOT block the loop.** Issue them off the loop via
  `World.Go`; their results re-enter the loop. The `Dispatcher` holds mutable state
  (e.g. wrong-password counters) lock-free precisely because it's only touched from the loop.
- `pMob[]` is a **shared index space**: indices `[0, MaxUser=1000)` are players, `[MaxUser, MaxMob)`
  are mobs/NPCs. Players and mobs share the `STRUCT_MOB` layout. Wire numbering depends on this.

### Request flow

`world.Server` accepts CPSock connections → `protocol` decodes/deobfuscates frames → `Dispatcher.Handle`
routes by message `Type` to a handler (`tmserver/internal/handler/dispatch.go` registers all routes,
grouped into "batches": login/char-select, movement, combat, items, trade, combine, party/guild,
chat/misc). Session state follows the `CUser.Mode` state machine (`UserEmpty`→`UserLogin`→
`UserSelChar`→`UserPlay`…) defined in `world/world.go`.

### Protocol & parity notes

- **No real encryption.** The CPSock layer is static-table obfuscation (`pKeyWord`) plus a
  **non-rejecting checksum**; the client's checks are NOP'd by the ClientPatch. `-reject-checksum` is
  **off by default** — only enable once a capture confirms correct checksums.
- **Binary layout is offset-explicit.** Wire structs use `pack(1)`; legacy save structs use
  **natural alignment** (MSVC x86). Don't rely on Go struct alignment for binary fidelity — read/write
  by explicit offset. Key sizes: `STRUCT_MOB`=816, `STRUCT_ACCOUNTFILE`=7952 (multiple save versions
  exist, distinguished by file size).
- **Exact RNG parity is a goal.** The MSVC `rand()` LCG is reimplemented in `tmserver/internal/rng`
  so drops/refines/crits match a controlled capture byte-for-byte. Preserve call order in gameplay code.
- Legacy passwords/PINs were plaintext; the rewrite hashes with **argon2id** (never persist plaintext).

### Content tree

tmServer loads and validates the `Release/` content tree at boot when `-content` is set (rates,
catalogs, maps, BaseMob templates, `serv00.htm`). Rates/catalogs are required (hard error if the mount
is broken); the large maps are optional (warn if absent). docker-compose mounts `./Release` read-only.

## Conventions

Follow `development-guidelines/Go-development-guidelines.md` (the project's authoritative Go style
guide). Highlights specific to this repo:

- `cmd/<bin>/main.go` does wiring only; private logic lives in `internal/`. Avoid generic
  `util`/`common` packages.
- Idiomatic Go naming: `MixedCaps`/`mixedCaps` (not `ALL_CAPS`), no stutter, getters without `Get`,
  `snake_case.go` filenames.
- Errors are the last return, wrapped with `%w` and context; never silently ignore (`_ = err` needs a
  justifying comment). `context.Context` is the first parameter.
- Comment the **why** (especially parity quirks), not the what. GoDoc on exported identifiers.
- Tests: table-driven, same package, run with `-race`. The protocol/world golden cases and
  transport vectors are the parity-critical tests.
- Pre-commit: `gofmt`/`goimports`, `go vet`, `golangci-lint run`, `go build ./...`, `go test -race ./...`,
  `govulncheck ./...` all clean.
- Secrets (DB DSN, etc.) come from the environment — never hardcode them.
