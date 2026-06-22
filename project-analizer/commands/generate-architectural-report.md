---
allowed-tools: Task, Read, Write, Edit, Bash, TodoWrite
description: Run a full architectural analysis including dependencies, architecture, and component deep-dives. Generate comprehensive reports with MANIFEST index.
---
# Project Architectural Analysis - Full Report Command

You MUST coordinate a multi-phase workflow invoking specialized agents and managing state via MANIFEST.md.

Extract the following parameters from command arguments:
- project-folder (optional, default: entire project root)
- output-folder (optional, default: "/docs/agents")
- ignore-folders (optional, comma-separated list)

---

## WORKFLOW EXECUTION

### Phase 1: Initialize Project Structure and MANIFEST

Create the base folder structure:
```
{output-folder}/
├── dependency-auditor/
├── architectural-analyzer/
├── component-deep-analyzer/
└── MANIFEST.md
```

Create MANIFEST.md using the Write tool with initial state:

```markdown
# MANIFEST - [PROJECT_NAME]
Generated on: {YYYY-MM-DD HH:MM:SS}

## Parameters
- Project folder: [PROJECT_FOLDER]
- Output folder: [OUTPUT_FOLDER]
- Ignore folders: [IGNORE_FOLDERS]

## Execution Status: IN_PROGRESS

## Reports

### dependency-auditor
- Status: PENDING
- Started: -
- Completed: -
- Output: -

### architectural-analyzer
- Status: PENDING
- Started: -
- Completed: -
- Output: -
- Components Found: -

### Components (to be populated after architectural analysis)
```

Replace [PROJECT_NAME] with the project name detected from folder or user input.
Replace [PROJECT_FOLDER], [OUTPUT_FOLDER], [IGNORE_FOLDERS] with actual values or defaults.

---

### Phase 2: Execute Dependency and Architecture Analysis in Parallel

Invoke both agents in parallel using the Task tool:

**For dependency-auditor:**

Pass the following prompt to the agent:

"Execute dependency audit with the following parameters:

Project scope: [PROJECT_SCOPE]
Output location: [OUTPUT_FOLDER]/dependency-auditor
Folders to ignore: [IGNORE_FOLDERS]

Execute your complete workflow following all internal guidelines.

CRITICAL REMINDERS:
- Do not modify any project files
- Use MCP servers (Context7, Firecrawl) for validation when available
- Always verify dependency versions externally
- Exclude folders specified in ignore-folders parameter from audit
- Save report to: [OUTPUT_FOLDER]/dependency-auditor/dependencies-report-{YYYY-MM-DD HH:MM:SS}.md

REPORT must include:
- All sections specified in agent guidelines
- Analysis of the 10 most critical files
- Explicit confirmation of saved file path
- List of unverified dependencies (if any)"

**For architectural-analyzer:**

Pass the following prompt to the agent:

"Execute architectural analysis with the following parameters:

Project scope: [PROJECT_SCOPE]
Output location: [OUTPUT_FOLDER]/architectural-analyzer
Folders to ignore: [IGNORE_FOLDERS]

Execute your complete workflow following all internal guidelines.

CRITICAL REMINDERS:
- Do not modify any project files
- Focus on architecturally significant components
- Identify ALL components in the Critical Components Analysis section
- Exclude folders specified in ignore-folders parameter from analysis
- Save report to: [OUTPUT_FOLDER]/architectural-analyzer/architectural-report-{YYYY-MM-DD HH:MM:SS}.md

REPORT must include:
- All sections specified in agent guidelines
- Complete list of components in Critical Components Analysis table
- Return the absolute path to saved file AND the list of all component names identified"

**After both agents complete:**

Update MANIFEST.md using the Edit tool:
- Set dependency-auditor status to COMPLETED with timestamp and output path
- Set architectural-analyzer status to COMPLETED with timestamp and output path
- Add all components found to the Components section with PENDING status

---

### Phase 3: Execute Component Deep Analysis in Parallel

Read the architectural report to extract the complete list of components from the Critical Components Analysis section.

For EACH component identified, invoke the component-deep-analyzer agent IN PARALLEL:

Pass the following prompt to each agent:

"Execute component deep analysis with the following parameters:

Component name: [COMPONENT_NAME]
Project scope: [PROJECT_SCOPE]
Output location: [OUTPUT_FOLDER]/component-deep-analyzer
Folders to ignore: [IGNORE_FOLDERS]

Execute your complete workflow following all internal guidelines.

CRITICAL REMINDERS:
- Analyze ONLY the component specified: [COMPONENT_NAME]
- Do not modify any project files
- Extract all business rules with detailed breakdown
- Locate and analyze test files across the project
- Exclude folders specified in ignore-folders parameter
- Save report to: [OUTPUT_FOLDER]/component-deep-analyzer/component-analysis-[COMPONENT_NAME]-{YYYY-MM-DD HH:MM:SS}.md

REPORT must include:
- All sections specified in agent guidelines
- Detailed breakdown of ALL business rules
- Test coverage analysis with test file locations
- Return the absolute path to saved file and component name"

Replace [COMPONENT_NAME] with the actual component name for each invocation.

**After each component agent completes:**

Update MANIFEST.md using the Edit tool:
- Set component status to COMPLETED with timestamp and output path
- If any component fails, set status to FAILED with error details

**Handling Failures:**

If any component analysis fails or times out:
- Update MANIFEST.md with FAILED status
- Continue with other components
- At the end of Phase 3, report failed components to user
- Offer to retry failed components using resume parameter if available

---

### Phase 4: Validate Completeness and Finalize MANIFEST

Read MANIFEST.md to verify all tasks are COMPLETED.

Use Bash tool to verify all report files exist at the specified paths:
```bash
ls [OUTPUT_FOLDER]/dependency-auditor/*.md
ls [OUTPUT_FOLDER]/architectural-analyzer/*.md
ls [OUTPUT_FOLDER]/component-deep-analyzer/*.md
```

Update MANIFEST.md with final status:
- If all tasks completed: Set "Execution Status: COMPLETED"
- If any tasks failed: Set "Execution Status: PARTIAL" and list failed tasks
- Add completion timestamp

---

### Phase 5: Generate Project Overview Summary

Create a comprehensive project overview report by reading all generated reports.

Use the Write tool to create: `[OUTPUT_FOLDER]/PROJECT-OVERVIEW-{YYYY-MM-DD HH:MM:SS}.md`

Template:

```markdown
# [PROJECT_NAME] - Project Overview

**Generated on**: {YYYY-MM-DD HH:MM:SS}

## Summary

[Brief 2-3 paragraph summary synthesizing key findings from all reports]

## Architecture Overview

[High-level architectural summary from architectural-analyzer report]

## Dependencies Health

[Summary of dependency status from dependency-auditor report - critical issues only]

## Components Analyzed

[List each component with 1-2 sentence summary of its purpose and key findings]

## Critical Findings

### Security Risks
[Aggregated security concerns from all reports]

### Technical Debt
[Aggregated technical debt items from all reports]

### Single Points of Failure
[Critical dependencies and architectural bottlenecks]

## Reports Index

See [MANIFEST.md](./MANIFEST.md) for complete list of all generated reports.
```

IMPORTANT:
- Do NOT include recommendations or action plans
- ONLY summarize findings from the reports
- Keep it factual and objective

---

### Phase 6: Generate README Index

Create the final README index file.

Use the Write tool to create: `[OUTPUT_FOLDER]/README-{YYYY-MM-DD HH:MM:SS}.md`

Template:

```markdown
# [PROJECT_NAME] - Architectural Analysis Reports

**Generated on**: {YYYY-MM-DD HH:MM:SS}

## Quick Links

- [Project Overview](./PROJECT-OVERVIEW-{YYYY-MM-DD HH:MM:SS}.md) - Executive summary of all findings
- [MANIFEST](./MANIFEST.md) - Complete registry of all reports

---

## Architecture and Dependencies

- [Project Architecture](./architectural-analyzer/architectural-report-{YYYY-MM-DD HH:MM:SS}.md)
- [Dependencies Report](./dependency-auditor/dependencies-report-{YYYY-MM-DD HH:MM:SS}.md)

## Component Analysis

[For each component, create a link:]
- [ComponentName](./component-deep-analyzer/component-analysis-ComponentName-{YYYY-MM-DD HH:MM:SS}.md)

---

## Workflow Execution

Analysis completed in [X] phases:
1. Dependency and Architecture Analysis (parallel)
2. Component Deep-Dive Analysis (parallel for [N] components)
3. Report Synthesis and Documentation

Total reports generated: [N]
```

Replace all placeholders with actual values from MANIFEST.md and generated reports.

Validate all links point to existing files using the Read tool.

---

## MANIFEST.md Management

Throughout the workflow, maintain MANIFEST.md as the single source of truth:

- **Use Edit tool** to update task statuses incrementally
- **Use Read tool** to check current state before updates
- **Include timestamps** for all status changes
- **Record agent IDs** if using resume functionality for long-running tasks
- **Log errors** for any failed tasks with enough detail to debug

---

## Error Handling and Recovery

If the workflow is interrupted:

1. Read MANIFEST.md to determine current state
2. Identify PENDING or FAILED tasks
3. Resume from the last incomplete phase
4. Use agent resume parameter if agent IDs were recorded
5. Continue execution without re-running completed tasks

If a specific agent fails:
1. Update MANIFEST.md with FAILED status and error message
2. Continue with other agents
3. Report failures to user at end of phase
4. Offer retry options

---

## Replace Placeholders

Before invoking agents, replace these placeholders:

- [PROJECT_NAME]: Detect from folder name or ask user
- [PROJECT_SCOPE]: Use `project-folder` parameter or "entire project root"
- [OUTPUT_FOLDER]: Use `output-folder` parameter or "/docs/agents"
- [IGNORE_FOLDERS]: Use `ignore-folders` parameter or "none"
- [COMPONENT_NAME]: Extract from architectural report for each component

---

## Usage Examples

```bash
# Analyze entire project with default output location
/generate-architectural-report

# Analyze specific folder
/generate-architectural-report --project-folder=src

# Custom output location
/generate-architectural-report --output-folder=reports/analysis

# Exclude folders from analysis
/generate-architectural-report --ignore-folders=node_modules,dist,test

# Combined usage
/generate-architectural-report --project-folder=src --output-folder=docs/reports --ignore-folders=node_modules,.git,dist
```
