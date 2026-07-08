# AGENTS.md

This file provides system-level context, instructions, and architectural constraints for AI Agents (e.g., Gemini CLI, Claude Code, Cursor, Copilot, etc.) working with this repository.

---

## 1. Project Core & Scope

This project is a **big-bang rewrite in Go** of the legacy WYD (With Your Destiny) MMORPG server, targeting the unmodified **`WYD.exe` 7662 client** (protocol version **7640** — 7662 is the build/patch name).

### Codebase Organization:
*   **Go Services (Current Rewrite):**
    *   `tmserver/`: Game server (owns world state, speaks CPSock).
    *   `dbserver/`: Persistence (PostgreSQL, pgx v5).
    *   `binserver/`: Billing server.
    *   `webserver/`: Web-API (Account creation, verification).
*   **Shared Packages:**
    *   `internal/`: Shared domain model, store/migration logic, secrets/hashing (Argon2id), and TLS.
*   **Legacy Code & Binaries:**
    *   `Source/`: Legacy C++ server sources (reference this to understand wire/format/gameplay behaviors).
    *   `Release/`: Runnable legacy binaries, game databases, maps, and server rates.
*   **Documentation:**
    *   `docs/migration/`: Extensive reverse-engineering documentation. **Consult this before making any modifications to protocol, wire formats, or game mechanics.**
    *   `docs/agents/`: Automated architectural analyses and reports for existing C++ components (see `docs/agents/MANIFEST.md` and `docs/agents/README-2026-06-19_16-06-38.md`).

---

## 2. Core Operational Commands

Agents should execute standard actions using the repository's `Makefile`:

```bash
make build          # Build all Go services
make binaries       # Build each service into bin/ (tmserver, dbserver, binserver, webserver)
make test           # Run all unit tests with race detection and coverage
make lint           # Run golangci-lint (gosec, staticcheck, govet, errcheck)
make vet            # Run go vet
make fmt            # Run gofmt and goimports
make vuln           # Check for vulnerabilities via govulncheck
make proto          # Regenerate gRPC code from api/*.proto
make certs          # Generate development mTLS certificates (ignored)
```

### Running Specific Tests:
```bash
go test -run <TestName> ./tmserver/internal/protocol
go test -tags=integration ./internal/store/...   # Run database integration tests
```

### Running the Stack:
```bash
make run            # Runs tmserver with no-op persistence
make run-local      # Runs full stack via docker compose + seeds 'test'/'test123' account
```

---

## 3. Crucial Architectural Invariants

### 🚨 The Single-Owner Game Loop (Absolute Invariant)
All world state is owned by **exactly one goroutine** (`world.World.Run`) and is never mutated anywhere else. **There must be NO locks on world state.**
*   **Parity:** This mirrors the single-threaded WinSock reactor of the original game and prevents item duplication or race conditions.
*   **Handlers:** Any handler under `tmserver/internal/handler/` runs inside the loop goroutine and can mutate world state directly without synchronization.
*   **Async Operations:** All blocking calls (e.g., database writes, billing validations) **must not block the loop**. Use `World.Go` to run them asynchronously, and pass their results back into the loop via channels.
*   **Shared Index Space:** `pMob[]` indices `[0, MaxUser=1000)` are players, and `[MaxUser, MaxMob)` are mobs/NPCs. They share the `STRUCT_MOB` layout.

### Legacy Protocol & Binary Conventions
*   **Static-Table Obfuscation:** The CPSock network layer uses static-table obfuscation (`pKeyWord`) and a non-rejecting checksum by default.
*   **Binary Alignment:** Legacy structures use **natural alignment (MSVC x86)**. Do not rely on Go's default struct alignment; read and write binary data using explicit offsets.
    *   `STRUCT_MOB` = 816 bytes
    *   `STRUCT_ACCOUNTFILE` = 7952 bytes (varies by save version)
*   **LCG RNG Parity:** For drop rates, item refinement, and crits, the MSVC `rand()` LCG is reimplemented in `tmserver/internal/rng`. Call order must be strictly preserved to maintain parity with original captures.

---

## 4. Key Agent Guidelines & Conventions

When modifying Go code, agents must adhere to the `development-guidelines/Go-development-guidelines.md`:

*   **Avoid Generic Utilities:** Do not create generic `util` or `common` packages. Use clean domain-driven structures.
*   **Naming Conventions:** Use idiomatic Go naming (`MixedCaps`/`mixedCaps`), no package name stuttering, and omit the `Get` prefix for getters. Filenames should be in `snake_case.go`.
*   **Error Handling:** Always return errors as the last value. Wrap them with `%w` to preserve context. Do not ignore errors silently; use a justifying comment if an error is explicitly ignored.
*   **Context:** `context.Context` should always be the first parameter in functions carrying context.
*   **Security:** Plaintext passwords or PINs are strictly prohibited. Always hash them using **Argon2id** (reusing the centralized implementation in `internal/secret`).
*   **Comments:** Comment the **why** (especially parity quirks, reverse engineering findings), not the what. Exported identifiers must have GoDoc-compliant comments.

---

## 5. Architectural Reference Material

Before making structural modifications, please consult:
1.  **Project Overview:** `docs/agents/PROJECT-OVERVIEW-2026-06-19_16-06-38.md`
2.  **Legacy Dependency Report:** `docs/agents/dependency-auditor/dependencies-report-2026-06-19_16-06-38.md`
3.  **Detailed Component Analyses:** `docs/agents/component-deep-analyzer/` for deep-dives on `Basedef`, `CPSock`, `TMSrv`, `DBSrv`, and `BISrv`.
