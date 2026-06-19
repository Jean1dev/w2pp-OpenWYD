# MANIFEST - w2pp-OpenWYD

Generated on: 2026-06-19 16:06:38

## Parameters
- Project folder: entire project root (source under `Source/Code`)
- Output folder: docs/agents
- Ignore folders: .git, .vs, Release, "serverlist bin", project-analizer, enc_temp_folder, *.rar archives, *.exe/*.dll/*.bin binaries

## Execution Status: COMPLETED
## Completed: 2026-06-19
## Note: Subagent session limit was reached during Phase 3; the remaining 8 component
## deep-dives were produced inline by the main thread (Basedef + dependency + architecture
## reports were produced by subagents before the limit).

## Reports

### dependency-auditor
- Status: COMPLETED
- Started: 2026-06-19 16:06:38
- Completed: 2026-06-19 16:10
- Output: docs/agents/dependency-auditor/dependencies-report-2026-06-19_16-06-38.md
- Note: 6 UNVERIFIED dependencies; Windows SDK / WinSock only, file-based persistence.

### architectural-analyzer
- Status: COMPLETED
- Started: 2026-06-19 16:06:38
- Completed: 2026-06-19 16:14
- Output: docs/agents/architectural-analyzer/architectural-report-2026-06-19_16-06-38.md
- Components Found: 24 candidates (consolidated to 9 cohesive components for deep-dive)

### Components (deep-dive scope; tightly-coupled files grouped)

1. Basedef — Status: COMPLETED — `Source/Code/Basedef.*` — component-analysis-Basedef-2026-06-19_16-06-38.md
8b. DBSrv — Status: COMPLETED — component-analysis-DBSrv-2026-06-19_16-06-38.md
2. CPSock — Status: COMPLETED — component-analysis-CPSock-2026-06-19_16-06-38.md
9b. BISrv — Status: COMPLETED — component-analysis-BISrv-2026-06-19_16-06-38.md
3. TMSrv-Core — Status: COMPLETED — component-analysis-TMSrv-Core-2026-06-19_16-06-38.md
4. TMSrv-CUser — Status: COMPLETED — component-analysis-TMSrv-CUser-2026-06-19_16-06-38.md
5. TMSrv-CItem — Status: COMPLETED — component-analysis-TMSrv-CItem-2026-06-19_16-06-38.md
6. TMSrv-CMob — Status: COMPLETED — component-analysis-TMSrv-CMob-2026-06-19_16-06-38.md
7. TMSrv-CastleWar — Status: COMPLETED — component-analysis-TMSrv-CastleWar-2026-06-19_16-06-38.md
8. DBSrv — Status: COMPLETED — component-analysis-DBSrv-2026-06-19_16-06-38.md
9. BISrv — Status: COMPLETED — component-analysis-BISrv-2026-06-19_16-06-38.md
