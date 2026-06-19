# Dependencies Audit Report — w2pp-OpenWYD

Date: 2026-06-19
Scope: Visual Studio solution/project configuration, Windows toolchain & SDK, linked system libraries, vendored third-party code, and runtime DLLs. No package manager exists for this project (no npm/pip/cargo).
Method: Static inspection only. No builds, installs, tests, or network calls were performed.

---

## 1. Executive Summary

w2pp-OpenWYD is a WYD ("With Your Destiny") MMORPG server emulator written in C++ for Windows, built with Microsoft Visual Studio (MSVC). The dependency surface is almost entirely the Windows platform itself:

- The build is driven by one solution file, `Source/Cavaleiros de Kersef.sln`, but that solution only references 3 of the 12 `.vcxproj` files present in the tree (`DBSrv`, `TMSrv`, `ClientPatch_v7662`). The other 9 projects are standalone editors/tools not wired into the solution.
- All linked libraries declared in the projects are standard Win32 system libraries from the Windows SDK (the default MSVC desktop-app linker template), including WinSock (`ws2_32.lib`), Winmm, and ODBC (`odbc32.lib`, `odbccp32.lib`).
- No external SDK/library is vendored as headers or import libs (no MySQL, OpenSSL, zlib, Boost, Lua, etc. found). Persistence is file-based (`CFileDB`), not database-backed, despite ODBC libs being on the linker line.
- One third-party binary DLL is committed into the repo: `Source/Code/ClientPatch_v7662/3da_extra9.dll` (a client-side PE patch component, UNVERIFIED origin).
- Runtime distribution under `Release/` ships Microsoft Visual C++ runtime redistributable DLLs (MSVC 2010/2013 era: `msvcr120`, `msvcp120`, `msvcr100d`, `ucrtbase`), which do not match the declared VS2022 (`v143`) toolset on the built projects.
- Toolchain is inconsistent across projects: PlatformToolset values `v143`, `v142`, and `v141` coexist; WindowsTargetPlatformVersion is `10.0` on some and `10.0.17763.0` on others.

---

## 2. Build System & Toolchain

Solution file: `Source/Cavaleiros de Kersef.sln`
- Format Version 12.00; `# Visual Studio Version 17`; `VisualStudioVersion = 17.2.32630.192` (Visual Studio 2022).
- Still carries legacy `GlobalSection(SubversionScc)` metadata (`Manager = AnkhSVN - Subversion Support for Visual Studio`).
- Solution references only 3 projects:
  - `DBSrv` — `Code\DBSrv\DBSrv.vcxproj`
  - `TMSrv` — `Code\TMSrv\TMSrv.vcxproj`
  - `ClientPatch_v7662` — `Code\ClientPatch_v7662\ClientPatch_v7662.vcxproj`
- Solution configurations declared: `Debug|Win32`, `Debug|x64`, `Release|Win32`, `Release|x64`, `Release-Themida|Win32`, `Release-Themida|x64`.
  - For `DBSrv` and `TMSrv`, every configuration maps its ActiveCfg to `Win32` only (x64 entries redirect to `Release|Win32`/`Debug|Win32`; no x64 build is actually produced).
  - `ClientPatch_v7662` is the only project with real `x64` build entries (`Debug|x64`, `Release|x64`).

The 12 `.vcxproj` projects found in `Source/Code/` (relative paths):

| Project | Path | ConfigurationType | PlatformToolset | WindowsTargetPlatformVersion | CharacterSet |
|---|---|---|---|---|---|
| TMSrv | `Source/Code/TMSrv/TMSrv.vcxproj` | Application | v143 | 10.0 | MultiByte |
| DBSrv | `Source/Code/DBSrv/DBSrv.vcxproj` | Application | v143 | 10.0 | MultiByte |
| BISrv | `Source/Code/BISrv/BISrv.vcxproj` | Application | v142 | 10.0 | MultiByte |
| ClientPatch_v7662 | `Source/Code/ClientPatch_v7662/ClientPatch_v7662.vcxproj` | Application / DynamicLibrary | v143 | 10.0 | MultiByte |
| ZerarSkill | `Source/Code/ZerarSkill/ZerarSkill.vcxproj` | Application | v141 | 10.0.17763.0 | MultiByte / Unicode |
| NPTool | `Source/Code/NPTool/NPTool.vcxproj` | Application | v141 | 10.0.17763.0 | MultiByte / Unicode |
| EDITAPPMOB | `Source/Code/EDITAPPMOB/EDITAPPMOB.vcxproj` | Application | v141 | 10.0.17763.0 | MultiByte / Unicode |
| EDITAPPSHOP | `Source/Code/EDITAPPSHOP/EDITAPPSHOP.vcxproj` | Application | v141 | 10.0.17763.0 | MultiByte |
| AttributeMap_Editor | `Source/Code/AttributeMap_Editor/AttributeMap_Editor.vcxproj` | Application | v141 | 10.0.17763.0 | Unicode |
| SearchPass | `Source/Code/SearchPass/SearchPass.vcxproj` | Application | v141 | 10.0.17763.0 | MultiByte / Unicode |
| DropTool | `Source/Code/DropTool/DropTool.vcxproj` | Application | v141 | 10.0.17763.0 | MultiByte / Unicode |
| ExpTool | `Source/Code/ExpTool/ExpTool.vcxproj` | Application | v141 | 10.0.17763.0 | MultiByte / Unicode |

Notes:
- Where two CharacterSet values appear, the project declares different sets per configuration (e.g., Debug vs Release).
- `ClientPatch_v7662` declares both `Application` and `DynamicLibrary` ConfigurationType across its configurations (it produces a DLL injected into the client).
- Target platform: effectively `Win32` (x86) for the two servers; `ClientPatch_v7662` additionally targets `x64`.
- Configurations: `Debug` and `Release` (plus the `Release-Themida` solution-level variant, which maps to `Release|Win32` for the servers).
- Language standard: No `<LanguageStandard>` element is declared in any project (defaults to the MSVC toolset default). `Source/Code/Basedef.h` includes C++11/14 headers (`<cstdint>`, `<unordered_map>`, `<array>`, `<tuple>`, `<functional>`, `<memory>`), so the effective standard is at least C++11.
- Runtime library: `TMSrv` uses `MultiThreadedDebug` (Debug) and `MultiThreadedDebugDLL`; `DBSrv` uses `MultiThreadedDebug`. (Static-vs-DLL CRT mix observed in declared values.)
- No `ProjectReference` elements and no shared `.props` property sheets exist; projects are independent.

---

## 3. System / Linked Libraries

The following are declared in `<AdditionalDependencies>` for the four projects that have a linker dependency list (`TMSrv`, `DBSrv`, `BISrv`, `NPTool`). The list is identical across all four and is the standard Visual Studio Win32 desktop-application linker default set.

No `#pragma comment(lib, ...)` directives were found anywhere in source.

| Library | Source | Where Referenced (relative path) | Purpose | Verified? |
|---|---|---|---|---|
| `Winmm.lib` | System (Windows SDK) | `Source/Code/TMSrv/TMSrv.vcxproj`, `Source/Code/DBSrv/DBSrv.vcxproj`, `Source/Code/BISrv/BISrv.vcxproj`, `Source/Code/NPTool/NPTool.vcxproj` | Windows multimedia (timers, `timeGetTime`) | Verified (declared) |
| `ws2_32.lib` | System (Windows SDK) | same four `.vcxproj` | WinSock 2 networking; backs the socket layer in `Source/Code/CPSock.cpp` / `CPSock.h` | Verified (declared + used) |
| `kernel32.lib` | System (Windows SDK) | same four `.vcxproj` | Core Win32 kernel API | Verified (declared) |
| `user32.lib` | System (Windows SDK) | same four `.vcxproj` | Windowing/message API (`WM_USER`-based socket events in `CPSock.h`) | Verified (declared + used) |
| `gdi32.lib` | System (Windows SDK) | same four `.vcxproj` | GDI graphics (default template) | Verified (declared) |
| `winspool.lib` | System (Windows SDK) | same four `.vcxproj` | Print spooler API (default template) | Verified (declared) |
| `comdlg32.lib` | System (Windows SDK) | same four `.vcxproj` | Common dialogs (default template) | Verified (declared) |
| `advapi32.lib` | System (Windows SDK) | same four `.vcxproj` | Registry/security/service API | Verified (declared) |
| `shell32.lib` | System (Windows SDK) | same four `.vcxproj` | Shell API (default template) | Verified (declared) |
| `ole32.lib` | System (Windows SDK) | same four `.vcxproj` | COM/OLE (default template) | Verified (declared) |
| `oleaut32.lib` | System (Windows SDK) | same four `.vcxproj` | OLE Automation (default template) | Verified (declared) |
| `uuid.lib` | System (Windows SDK) | same four `.vcxproj` | GUID/UUID symbols; `Basedef.h` includes `<Rpc.h>` | Verified (declared) |
| `odbc32.lib` | System (Windows SDK) | same four `.vcxproj` | ODBC database access (default template) — no ODBC API calls found in source | Verified (declared); UNVERIFIED whether used |
| `odbccp32.lib` | System (Windows SDK) | same four `.vcxproj` | ODBC installer API (default template) — no ODBC API calls found in source | Verified (declared); UNVERIFIED whether used |

Additional system headers included directly in shared code (`Source/Code/Basedef.h`): `<Windows.h>`, `<WinSock.h>` (WinSock 1.1 header), `<Rpc.h>`, `<mbstring.h>`, plus standard C/C++ library headers. `Source/Code/CPSock.h` includes `<Windows.h>` and defines the socket abstraction (`SOCKET`, `WSAInitialize`, `StartListen`, `ConnectServer`, `closesocket` wrappers) on a `WM_USER`/`WSAAsyncSelect`-style asynchronous socket model.

Observation: `Basedef.h` includes the legacy `<WinSock.h>` (WinSock 1.1) while the linker pulls `ws2_32.lib` (WinSock 2). This is a header/lib version mismatch but both are part of the system SDK.

Database libraries (`odbc32`, `odbccp32`) are linked but no SQL/ODBC API calls (`SQLConnect`, `SQLDriverConnect`, `SQLExecDirect`, `SQLAllocHandle`, `#include <sql.h>`) were found in `TMSrv`, `DBSrv`, or `BISrv`. Persistence is implemented file-based via `Source/Code/DBSrv/CFileDB.cpp` / `CFileDB.h`. The ODBC libs are therefore the unused remainder of the default MSVC project template.

---

## 4. Vendored / Bundled Third-Party Code

- `Source/Code/ClientPatch_v7662/3da_extra9.dll` — a committed, prebuilt 32-bit Windows PE DLL (`PE32 executable (DLL) (GUI) Intel 80386`, 5 sections, ~614 KB). Tracked in git. It sits inside the client-patch project (which hooks the WYD client via `PE_Hook.h` / `Hook.cpp`). Origin, version, and build provenance are UNVERIFIED; no source for it is present and no project references it by name.

No vendored third-party source files or headers (e.g., MinHook/Detours, zlib, MySQL connector, OpenSSL, Lua, JSON libraries) were found in the tree. The client hook uses a self-contained `Source/Code/ClientPatch_v7662/PE_Hook.h` rather than an external hooking library.

The `.rar` archives at repo root (`Conversor.rar`, `Ferramentas.rar`) and the `serverlist bin`/`project-analizer`/`enc_temp_folder` directories were excluded per scope and not inspected.

---

## 5. Runtime DLLs Shipped

DLLs present under `Release/` (all under `Release/TMsrv/run/`; tracked in git):

| DLL | Subsystem (best-effort) | Notes |
|---|---|---|
| `msvcr120.dll` | Visual C++ 2013 CRT (C runtime) | Release C runtime, MSVC v120 (VS2013) |
| `msvcp120.dll` | Visual C++ 2013 CRT (C++ standard library) | Release C++ runtime, MSVC v120 (VS2013) |
| `msvcr120d.dll` | Visual C++ 2013 CRT (debug) | Debug C runtime, MSVC v120 |
| `msvcp120d.dll` | Visual C++ 2013 CRT (C++, debug) | Debug C++ runtime, MSVC v120 |
| `msvcr100d.dll` | Visual C++ 2010 CRT (debug) | Debug C runtime, MSVC v100 (VS2010) — older than the others |
| `ucrtbase.dll` | Universal C Runtime (UCRT) | Windows 10 universal CRT component |

Identification is by conventional Microsoft DLL naming; exact file versions were not extracted (no Windows tooling run). The shipped runtimes (v100/v120/UCRT) do not correspond to the declared build toolsets (`v143` = VS2022 for the servers, `v141`/`v142` for the tools), indicating the runtime set was carried over from an earlier toolchain or hand-assembled. The debug CRTs (`*d.dll`) are not redistributable and being present in a `Release` folder is anomalous.

No other DLLs ship under `Release/`. The remaining `Release/` contents are server executables (`TMSrv.exe`, `DBSrv.exe`, tool `.exe`s), `.pdb`, and game data/config text/binary files.

---

## 6. Critical Files Reviewed

1. `Source/Cavaleiros de Kersef.sln` — VS2022 solution; only wires `DBSrv`, `TMSrv`, `ClientPatch_v7662`; 6 solution configurations incl. `Release-Themida`; legacy AnkhSVN/Subversion SCC section; x64 entries for the servers redirect to Win32.
2. `Source/Code/TMSrv/TMSrv.vcxproj` — main game/world server. `v143`, SDK `10.0`, MultiByte, Win32. Standard Win32 linker dep set. Mixed RuntimeLibrary (`MultiThreadedDebug` / `MultiThreadedDebugDLL`).
3. `Source/Code/DBSrv/DBSrv.vcxproj` — database/account server (file-backed, `CFileDB`). `v143`, SDK `10.0`, MultiByte, Win32. Same linker dep set; `MultiThreadedDebug`.
4. `Source/Code/BISrv/BISrv.vcxproj` — billing server. `v142` (older toolset than the two servers in the solution), SDK `10.0`. Same linker dep set. Not referenced by the solution.
5. `Source/Code/ClientPatch_v7662/ClientPatch_v7662.vcxproj` — client-side patch/hook. `v143`, builds both Application and DynamicLibrary configs, targets Win32 and x64. No `<AdditionalDependencies>` declared. Bundles `3da_extra9.dll`.
6. `Source/Code/NPTool/NPTool.vcxproj` — tool with full Win32 linker dep set; `v141`, SDK `10.0.17763.0`.
7. `Source/Code/Basedef.h` (2920 lines) — shared core header. Includes `<Windows.h>`, `<WinSock.h>` (WinSock 1.1), `<Rpc.h>`, `<cstdint>`, and a broad set of C++ STL headers; defines the game's packet/structure layouts (`#pragma pack(push,1)`); no third-party includes.
8. `Source/Code/CPSock.cpp` (692 lines) / `Source/Code/CPSock.h` (134 lines) — WinSock networking abstraction (`SOCKET` wrappers: `WSAInitialize`, `StartListen`, `ConnectServer`, `ConnectBillServer`, `CloseSocket`, send/recv buffers of 128 KB), event-driven via `WM_USER` message codes. Confirms `ws2_32.lib`/`user32.lib` usage.
9. `Source/Code/DBSrv/CFileDB.cpp` / `CFileDB.h` — confirms file-based persistence (no SQL), explaining the unused ODBC libs.
10. `Source/Code/ClientPatch_v7662/Main.h` / `PE_Hook.h` / `Hook.cpp` — client PE-hooking; includes only standard headers plus the local `PE_Hook.h` and `..\Basedef.h`; no external hooking-library dependency.

No common/shared property sheet (`.props`) exists; each project carries its own settings.

---

## 7. Risk Notes (observations only)

- Toolchain inconsistency: PlatformToolset spans `v141` (VS2017), `v142` (VS2019), and `v143` (VS2022) across the 12 projects; WindowsTargetPlatformVersion spans `10.0` and `10.0.17763.0`. The solution-built servers are `v143` while `BISrv` is `v142` and all 9 standalone tools are `v141`.
- Runtime/toolset mismatch: shipped CRTs are MSVC v100 (2010) and v120 (2013) plus UCRT, none matching the declared `v143`/`v142`/`v141` build toolsets. The actual runtime the binaries link against was not verified from the PE headers.
- Debug CRTs in a Release folder: `msvcr100d.dll`, `msvcr120d.dll`, `msvcp120d.dll` are non-redistributable debug runtimes present under `Release/`.
- WinSock header/lib split: `<WinSock.h>` (1.1) included in `Basedef.h` while `ws2_32.lib` (WinSock 2) is linked.
- Unused database libs: `odbc32.lib` / `odbccp32.lib` are linked with no corresponding ODBC code; persistence is file-based.
- MultiByte character set + no explicit `<LanguageStandard>`: effective C++ standard is toolset-default and not pinned.
- Committed opaque binary: `3da_extra9.dll` is a prebuilt third-party/unknown-provenance DLL tracked in source control with no accompanying source or hash.
- Legacy SCC metadata: the solution still declares AnkhSVN/Subversion management while the repo is Git.
- 9 of 12 projects are not part of the solution build graph, so their toolchain/SDK requirements are not exercised by the main build.

---

## 8. Unverified Dependencies

1. `3da_extra9.dll` (`Source/Code/ClientPatch_v7662/3da_extra9.dll`) — origin, vendor, version, and purpose UNVERIFIED.
2. Exact file versions of all shipped runtime DLLs (`msvcr120.dll`, `msvcp120.dll`, `msvcr120d.dll`, `msvcp120d.dll`, `msvcr100d.dll`, `ucrtbase.dll`) — UNVERIFIED (not extracted from PE metadata).
3. Whether `odbc32.lib` / `odbccp32.lib` are actually required at link time — UNVERIFIED (declared in template, no source usage found).
4. The actual MSVC CRT version the built `.exe`s link against — UNVERIFIED (no PE import-table inspection performed).
5. Contents/dependencies of excluded archives `Conversor.rar` and `Ferramentas.rar` at repo root — UNVERIFIED (out of scope).
6. The "Themida" referenced by the `Release-Themida` solution configuration (a commercial packer/protector) — no Themida tooling, license, or binary located in-scope; integration UNVERIFIED.

Total UNVERIFIED dependencies/items: 6
