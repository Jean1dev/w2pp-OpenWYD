# Component Deep Analysis Report: CPSock

**Component**: CPSock (Packet / Socket Networking Layer)
**Project**: w2pp-OpenWYD
**Generated**: 2026-06-19 16:06:38
**Scope**: `Source/Code/CPSock.cpp` (692 lines), `Source/Code/CPSock.h` (135 lines)

---

## 1. Executive Summary

CPSock is the shared transport layer used by every server in the stack (TMSrv, DBSrv, BISrv)
and is the boundary where raw WinSock bytes become structured game messages. It implements:
listen/connect socket setup, a per-connection receive/send ring of large flat buffers, message
framing against a fixed 12-byte `HEADER`, a table-based byte-obfuscation scheme, and an additive
checksum.

Two findings dominate the security posture of this layer:

1. **Checksum failures do not reject packets.** In `ReadMessage`, when the computed sum differs
   from the header checksum, the code sets `*ErrorCode = 1` and then `return pMsg;` anyway — the
   decoded message is handed back to the caller regardless (`CPSock.cpp:458-466`, comment at
   `:464` reads "return packet, even check_sum not match"). Integrity therefore depends entirely
   on each caller honoring `ErrorCode`, which is inconsistent across the codebase.
2. **There is no real encryption.** Confidentiality/obfuscation relies on a static 512-byte
   table `pKeyWord` committed in source (`CPSock.cpp:29`, labelled "7.xx keys"). Each packet
   picks a random index byte (`iKeyWord = rand()%256`) and applies a fixed positional
   add/subtract-with-shift transform keyed off that table. This is a reversible scramble whose
   key material is public in the repository, not cryptographic protection.

On the positive side, the framing path does validate message length against `MAX_MESSAGE_SIZE`
(8192) and `sizeof(HEADER)`, and checks that the full declared size has arrived before exposing
the message, which bounds the transform loop. An `INITCODE` (0x1F11F311) handshake gates the
first bytes of every connection.

---

## 2. Data Flow Analysis

Receive path:
```
1. WinSock FD_READ event -> CPSock::Receive() (CPSock.cpp:336)
2. recv() appends into pRecvBuffer at nRecvPosition (CPSock.cpp:339)
3. Caller loops CPSock::ReadMessage() (CPSock.cpp:353):
   a. On a fresh connection, first 4 bytes must equal INITCODE 0x1F11F311 (CPSock.cpp:368-381)
   b. Require >= sizeof(HEADER) bytes buffered
   c. Parse Size, iKeyWord, CheckSum, Type, ID from header
   d. Reject if Size > MAX_MESSAGE_SIZE(8192) or Size < sizeof(HEADER) -> ErrorCode 2
   e. Require full Size bytes present, else wait
   f. De-obfuscate bytes [4..Size) using pKeyWord table + positional transform
   g. Compute additive checksum; if mismatch set ErrorCode=1 but STILL return pMsg
4. Caller dispatches the returned message (e.g., ProcessClientMessage / ProcessMessage)
```

Send path:
```
1. Caller builds a message struct and calls CPSock::AddMessage(pMsg, Size) (CPSock.cpp:513)
2. Buffer-full and invalid-socket guards (CPSock.cpp:518-531)
3. Random iKeyWord; header Size/KeyWord/CheckSum/ClientTick stamped (CPSock.cpp:533-539)
4. Bytes [4..Size) obfuscated into pSendBuffer; checksum = Sum2 - Sum1 (CPSock.cpp:558-584)
5. First 4 header bytes memcpy'd into send buffer (CPSock.cpp:586)
6. CPSock::SendMessageA() flushes pSendBuffer via send() (CPSock.cpp:617,662)
```

---

## 3. Business Rules & Logic

### Overview

| Rule Type | Rule Description | Location |
|-----------|------------------|----------|
| Handshake | First 4 bytes of a connection must equal INITCODE 0x1F11F311 | `CPSock.cpp:373-381` |
| Validation | Message Size must be within [sizeof(HEADER), MAX_MESSAGE_SIZE(8192)] | `CPSock.cpp:398-407` |
| Framing | Wait until full declared Size has arrived before returning a message | `CPSock.cpp:410-412` |
| Obfuscation | Positional table transform over bytes [4..Size) keyed by pKeyWord | `CPSock.cpp:430-451`, `:558-581` |
| Integrity | Additive checksum `Sum2 - Sum1` compared to header CheckSum | `CPSock.cpp:455-458`, `:583` |
| Integrity gap | On checksum mismatch the packet is still returned (ErrorCode=1) | `CPSock.cpp:458-466` |
| Resource | RECV/SEND buffers 128 KB; max single message 8 KB | `CPSock.h:35-38` |
| Resource | Send rejected if it would overflow SEND_BUFFER_SIZE | `CPSock.cpp:518-524` |

### Detailed breakdown

---

### Business Rule: Connection Init Handshake (INITCODE)

**Overview**: Every new connection must begin with the 4-byte constant `INITCODE` (0x1F11F311)
before any message is accepted.

**Detailed description**: When `Init == 0`, `ReadMessage` requires at least 4 buffered bytes and
reads them as an `unsigned int`. If they do not equal `INITCODE` it returns `ErrorCode = 2` and
reports the offending value in `ErrorType` (`CPSock.cpp:373-381`). On success `Init` is set to 1
and the 4 bytes are consumed (`nProcPosition += 4`). The same constant is written by
`ConnectServer` via `send(tSock, &InitCode, 4, 0)` (`CPSock.cpp:250`). This acts as a cheap
protocol/version gate and filter against non-WYD traffic, but is a fixed public constant and is
not an authentication mechanism.

**Rule workflow**: new conn → need 4 bytes → compare to INITCODE → accept (Init=1) or reject (ErrorCode 2).

---

### Business Rule: Message Length Validation and Framing

**Overview**: The layer guarantees a returned message is a complete, length-bounded frame.

**Detailed description**: After confirming at least a header is present, it reads the `Size`
field and rejects any `Size > MAX_MESSAGE_SIZE` (8192) or `Size < sizeof(HEADER)`, resetting the
buffer positions and returning `ErrorCode = 2` with the bad size (`CPSock.cpp:398-407`). It then
checks `Size <= Rest` (bytes actually buffered) and waits if the frame is incomplete
(`CPSock.cpp:410-412`). Only then is the message pointer exposed and `nProcPosition` advanced.
Because the de-obfuscation/checksum loop iterates `i` from 4 to `Size`, and `Size` is bounded
and confirmed present, the loop stays within the received bytes. These bounds checks are the
main memory-safety guard of the transport layer.

**Rule workflow**: header present → bound-check Size → ensure full frame buffered → expose frame.

---

### Business Rule: Table-Based Obfuscation (pKeyWord transform)

**Overview**: Message payload bytes are scrambled/unscrambled with a static 512-entry table and
a position-dependent arithmetic transform.

**Detailed description**: `pKeyWord[512]` is a constant table compiled into the binary
(`CPSock.cpp:29`). For each packet a random index `iKeyWord = rand()%256` is chosen; the header
stores `iKeyWord`, and `KeyWord = pKeyWord[iKeyWord*2]` seeds a running position. For each
payload byte `i` in `[4, Size)`, `Trans = pKeyWord[(pos%256)*2 + 1]` and the byte is modified by
one of four operations selected by `i & 0x3`: subtract `Trans<<1`, add `Trans>>3`, subtract
`Trans<<2`, or add `Trans>>5` (decode at `CPSock.cpp:436-451`; the inverse add/sub on encode at
`CPSock.cpp:564-581`). Because the table is fixed and shipped in the source tree, this provides
obfuscation against casual inspection only; an attacker with the repository can fully reverse it.
It is documented here as the de facto "encryption" of the protocol, not as a cryptographic
control.

**Rule workflow**: pick random index → seed position from table → per-byte add/sub by `i&3` → emit transformed bytes.

---

### Business Rule: Additive Checksum (non-rejecting)

**Overview**: A one-byte checksum is computed over the payload, but a mismatch does not stop the
message from being processed.

**Detailed description**: During decode, `Sum2` accumulates the still-obfuscated bytes and `Sum1`
accumulates the de-obfuscated bytes; the validation value is `Sum = Sum2 - Sum1`
(`CPSock.cpp:455`). On encode the stored `CheckSum = Sum2 - Sum1` is written to the header
(`CPSock.cpp:583-584`). If `Sum != CheckSum` on receive, the code sets `*ErrorCode = 1` and then
executes `return pMsg;` — i.e., it returns the message anyway (`CPSock.cpp:458-466`). The comment
on `:464` states this explicitly. Thus checksum enforcement is delegated to callers: BISrv's loop
breaks on `Error == 1 || Error == 2` (`BISrv/Main.cpp:92`), but other call sites may not, so the
integrity guarantee is inconsistent across servers.

**Rule workflow**: decode payload → compute Sum → compare to header CheckSum → on mismatch set ErrorCode=1 but return packet.

---

## 4. Component Structure

```
CPSock.h          # HEADER struct (12 bytes), CPSock class, WSA_* event ids,
                  #   buffer-size and INITCODE constants, _AUTH_GAME placeholder
CPSock.cpp
├── pKeyWord[512]            # static obfuscation key table ("7.xx keys")  (:29)
├── WSAInitialize/CloseSocket
├── StartListen             # bind/listen for a server socket             (:128)
├── ConnectServer / ConnectBillServer  # outbound links; send INITCODE    (:176,:257)
├── Receive                 # recv() into pRecvBuffer                      (:336)
├── ReadMessage             # framing + de-obfuscation + checksum          (:353)
├── ReadBillMessage         # billing-link variant                        (:469)
├── AddMessage              # enqueue + obfuscate + checksum               (:513)
├── SendMessageA / SendOneMessage / SendBillMessage  # flush via send()
└── RefreshRecv/SendBuffer  # compaction of flat buffers
```

`HEADER` layout (`CPSock.h:42-50`): `short Size; char KeyWord; char CheckSum; short Type;
short ID; unsigned int ClientTick;` — 12 bytes; the obfuscation/checksum loop starts at offset 4
(i.e., it does not transform `Size`/`KeyWord`/`CheckSum`).

## 5. Dependency Analysis

```
Internal:
  CPSock ──> HEADER / message structs (Basedef.h), Log() (host server)
  TMSrv, DBSrv, BISrv ──> CPSock (each connection embeds a CPSock cSock)

External:
  - WinSock (ws2_32): socket, bind, listen, accept, connect, recv, send, WSAAsyncSelect
  - Windows API: WM_USER-based WSA_* messages routed by the host window proc
  - C runtime: rand(), memcpy, sprintf for logging
```

## 6. Afferent and Efferent Coupling

Afferent coupling counts the units that depend on CPSock; efferent counts what CPSock depends
on. Estimates from `#include`/usage: CPSock is included by all three servers and embedded in
every connection object, giving it very high afferent coupling; its own dependencies are narrow.

| Unit | Afferent (est.) | Efferent (est.) | Critical |
|------|-----------------|-----------------|----------|
| CPSock (class) | Very High (all servers, every connection) | WinSock, Basedef HEADER, Log | High |
| pKeyWord table | High (every encode/decode) | none | Medium |
| HEADER struct | Very High (all message structs derive layout) | none | High |

## 7. Integration Points

| Integration | Type | Purpose | Protocol | Data Format | Error Handling |
|-------------|------|---------|----------|-------------|----------------|
| Game client link | TCP | Client <-> TMSrv messages | WinSock, INITCODE-gated | obfuscated `HEADER`+payload | length/INITCODE checks; checksum non-rejecting |
| Inter-server link | TCP | TMSrv <-> DBSrv | WinSock | same framing | same |
| Billing link | TCP | TMSrv/BISrv | `ConnectBillServer` / `ReadBillMessage` | same framing | same |

## 8. Design Patterns & Architecture

| Pattern | Implementation | Location | Purpose |
|---------|----------------|----------|---------|
| Reactor / event-driven IO | WSA_* window messages + Receive/ReadMessage | `CPSock.h:25-31` | Non-blocking single-thread socket IO |
| Flat ring/compaction buffers | pRecvBuffer/pSendBuffer + position indices | `CPSock.cpp` | Stream reassembly without per-message alloc |
| Fixed-header framing | `HEADER` + Size field | `CPSock.h:42` | Delimit variable-length messages on a stream |
| Static-table obfuscation | pKeyWord transform | `CPSock.cpp:29,436` | Lightweight payload scrambling |

## 9. Technical Debt & Risks

| Risk Level | Area | Issue | Impact |
|------------|------|-------|--------|
| Critical | Integrity | Checksum mismatch returns the packet anyway (`CPSock.cpp:464`) | Corrupted/forged packets can be processed if caller ignores ErrorCode |
| High | Confidentiality | "Encryption" is a static table shipped in source (`CPSock.cpp:29`) | Protocol fully reversible by anyone with the repo |
| High | Auth | INITCODE is a fixed public constant, not authentication | No protection against crafted clients |
| Medium | Robustness | Inconsistent caller handling of ErrorCode 1/2 across servers | Divergent behavior on malformed input |
| Low | Memory safety | Transform indexes payload by header Size | Bounded by validated Size/Rest checks (mitigated) |

## 10. Test Coverage Analysis

No automated tests exist for CPSock or any other module in the repository (no test project,
framework, or `*test*` sources). The framing, transform, and checksum logic — the highest-risk
code in the project — is entirely unverified by automated tests. Recorded as a coverage risk.

| Component | Unit Tests | Integration Tests | Coverage | Notes |
|-----------|------------|-------------------|----------|-------|
| CPSock | 0 | 0 | None | No tests present in repo |
