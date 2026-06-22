# Component Deep Analysis Report: BISrv

**Component**: BISrv (Billing Server)
**Project**: w2pp-OpenWYD
**Generated**: 2026-06-19 16:06:38
**Scope**: `Source/Code/BISrv/` (analysis and reporting only)

---

## 1. Executive Summary

BISrv is nominally the "Billing Server" of the OpenWYD stack, intended to handle cash-shop /
donate transactions. In its current state it is a **skeleton / non-functional stub**: the
entire module is 385 lines, and its message handler `ProcessMessage` performs no work at all.

The defining observation of this analysis: `ProcessMessage(char *pMsg)` (`ProcessMessage.cpp:25-28`)
casts the inbound buffer to a `HEADER*` and then returns without dispatching, validating,
crediting, debiting, or persisting anything. There is no `switch` on message type, no balance
logic, no item delivery, and no persistence. Consequently BISrv implements **zero billing
business rules**.

What does exist is the Win32 host scaffolding: a dialog-based message pump (`WinMain` →
`MainDlgProc`, `Main.cpp:36,64`), a single `CUser Admin` connection serviced over WinSock
(`WSA_READ` event), a logging facility writing to `.\Log\BI_*.txt`, and a call to
`BASE_InitializeServerList()` at startup. The real cash/coin economy is implemented elsewhere:
DBSrv owns the account `Coin` balance and the `_MSG_DBUpdateSapphire` handler, and TMSrv drives
donate/cash-item flows. BISrv is also notable for being a standalone project not wired into the
main solution (per the architectural report) and for the hardcoded billing IP shipped in
`Release/TMsrv/run/biserver.txt`.

---

## 2. Data Flow Analysis

```
1. WinMain creates the dialog host and message loop (Main.cpp:36-62)
2. BASE_InitializeServerList() loads server-list config at startup (Main.cpp:47)
3. A single Admin connection (CUser Admin) is serviced on WSA_READ (Main.cpp:68)
4. Admin.cSock.Receive() pulls bytes; ReadMessage() frames a message (Main.cpp:78-87)
5. ProcessMessage(Msg) is invoked ... and does nothing (ProcessMessage.cpp:25-28)
6. No balance check, no item grant, no persistence, no reply is produced
```

The data-flow effectively terminates at step 5; there is no downstream processing to map.

---

## 3. Business Rules & Logic

### Overview

| Rule Type | Rule Description | Location |
|-----------|------------------|----------|
| (none implemented) | `ProcessMessage` is an empty stub; no billing rules exist | `ProcessMessage.cpp:25-28` |
| Infrastructure | Single `Admin` socket; closes on non-FD_READ event | `Main.cpp:68-74` |
| Infrastructure | Daily log file `.\Log\BI_<date>.txt` | `Main.cpp:142-157` |

### Detailed breakdown

---

### Business Rule: (Absent) Billing / Cash Transaction Processing

**Overview**: BISrv is expected to validate and apply cash-shop purchases, but no such logic
is present in the current source.

**Detailed description**: The only message entry point, `ProcessMessage`, reads the message
header type into `std` and immediately returns (`ProcessMessage.cpp:27`). There is no
dispatch table, no purchase validation, no balance debit/credit, no item delivery, and no
acknowledgement back to TMSrv. As a result, any billing-related behavior a deployment relies
on must be coming from another component, not BISrv.

In practice the cash economy is realized in the other servers: DBSrv carries the per-account
`Coin` field and clamps it to non-negative on login (`DBSrv/CFileDB.cpp:646`) and exposes
`_MSG_DBUpdateSapphire` (`DBSrv/CFileDB.cpp` switch), while TMSrv mediates donate/cash-item
delivery to players. BISrv, as committed, is a placeholder.

**Rule workflow**: message arrives → header cast → return (no-op).

---

### Business Rule: Admin Connection Handling (the only live behavior)

**Overview**: BISrv maintains one administrative socket and drains framed messages from it.

**Detailed description**: On the `WSA_READ` window message, the handler checks the select
event is `FD_READ`; otherwise it closes the socket (`Main.cpp:70-74`). It records
`CurrentTime = timeGetTime()`, calls `Admin.cSock.Receive()`, and loops `ReadMessage()` to
frame each message, logging WinSock errors with code (`Main.cpp:92-98`) before handing each
to the no-op `ProcessMessage`. This is the only functional runtime path.

**Rule workflow**: WSA_READ → verify FD_READ → Receive → loop ReadMessage → ProcessMessage (no-op).

---

## 4. Component Structure

```
Source/Code/BISrv/
├── Main.cpp           # 157 lines: WinMain message pump, MainDlgProc (WSA_READ), Log, StartLog
├── Main.h             # includes, manifest-dependency pragmas, decls
├── ProcessMessage.cpp #  27 lines: ProcessMessage stub (no dispatch)
├── ProcessMessage.h   # declaration
├── CUser.cpp          #  37 lines: CUser ctor/dtor (connection state)
├── CUser.h            # CUser class (Mode, IP, Count, Encode1/2, cSock)
└── resource.h         # Win32 dialog resource ids
```

## 5. Dependency Analysis

```
Internal:
  Main ──> CUser (Admin), ProcessMessage, CPSock (via CUser.cSock)
  Main ──> BASE_InitializeServerList (shared Basedef/base helpers)

External:
  - WinSock (ws2_32) — single Admin socket, WSAAsyncSelect-style WSA_READ
  - Windows API / Common Controls — dialog host (CreateDialogParam, MainDlgProc)
  - Winmm (timeGetTime) — timestamping
  - C runtime / filesystem — log files
```

## 6. Afferent and Efferent Coupling

Afferent coupling = units depending on a component; efferent = what it depends on. Estimates
from the (small) `#include` graph and symbol usage in `Source/Code/BISrv/`.

| Component | Afferent (est.) | Efferent (est.) | Critical |
|-----------|-----------------|-----------------|----------|
| Main (host) | entry point | CUser, ProcessMessage, CPSock, Basedef | Medium |
| ProcessMessage | Main | HEADER (Basedef) only | Low (stub) |
| CUser | Main | CPSock | Low |

## 7. Endpoints

No functional endpoints are exposed. `ProcessMessage` does not dispatch any message type, so
there is no billing protocol surface to document at present.

## 8. Integration Points

| Integration | Type | Purpose | Protocol | Data Format | Error Handling |
|-------------|------|---------|----------|-------------|----------------|
| Admin socket | Internal | Receives framed messages (then discarded) | TCP / WSA_READ | CPSock binary | logs WSA error code; closes on non-FD_READ |
| Server list config | Filesystem | `BASE_InitializeServerList` at startup | File IO | text/binary | none observed |
| Billing IP (deployment) | Config | TMSrv points at BISrv via hardcoded IP | text | `Release/TMsrv/run/biserver.txt` | none |
| Log files | Filesystem | `.\Log\BI_<date>.txt` | File IO | text | append; reopened daily |

## 9. Design Patterns & Architecture

| Pattern | Implementation | Location | Purpose |
|---------|----------------|----------|---------|
| Dialog message pump | `WinMain` + `MainDlgProc` | `Main.cpp:36,64` | Win32 GUI host & event loop |
| Reactor (single socket) | `WSA_READ` + `Receive`/`ReadMessage` | `Main.cpp:68-101` | Async message intake |
| Null Object (de facto) | empty `ProcessMessage` | `ProcessMessage.cpp:25` | Placeholder for unimplemented logic |

## 10. Technical Debt & Risks

| Risk Level | Area | Issue | Impact |
|------------|------|-------|--------|
| High | Functionality | `ProcessMessage` is a no-op; billing is unimplemented | Any feature relying on BISrv silently does nothing |
| Medium | Deployment | Hardcoded billing IP in `Release/TMsrv/run/biserver.txt` | Environment-coupled config; exposure if repo public |
| Medium | Architecture | Standalone project not in main solution | Build/runtime drift vs other servers |
| Low | Robustness | Single Admin socket, no reconnection/back-pressure logic | Limited operability |

## 11. Test Coverage Analysis

No automated tests exist for BISrv (or anywhere in the repository — no test project, framework,
or `*test*` sources). Given that the core handler is an empty stub, there is also no behavior
to test at present. Recorded as a coverage gap.

| Component | Unit Tests | Integration Tests | Coverage | Notes |
|-----------|------------|-------------------|----------|-------|
| ProcessMessage | 0 | 0 | None | Stub; nothing to cover |
| Main / CUser | 0 | 0 | None | No tests present in repo |
