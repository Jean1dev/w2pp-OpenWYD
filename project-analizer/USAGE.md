# Project Analyzer Plugin

## Objective

The Project Analyzer plugin provides comprehensive architectural analysis capabilities for software projects. It includes three specialized agents: architectural analysis, component deep-dive analysis, and dependency auditing. These tools help teams understand system architecture, component details, and dependency health without modifying the codebase.

## Installation

### Prerequisites

First, add the marketplace to Claude Code:

```bash
/plugin marketplace add devfullcycle/claude-mkt-place
```

### Install Plugin

Then install the Project Analyzer plugin:

```bash
/plugin install project-analizer@devfullcycle
```

## Available Commands

### `/generate-architectural-report`

**Description**: Runs a complete architectural analysis workflow including dependency auditing, architectural analysis, and component deep-dive analysis. Generates comprehensive reports with a MANIFEST index tracking all generated reports.

**Syntax**:
```bash
/generate-architectural-report [--project-folder=PATH] [--output-folder=PATH] [--ignore-folders=LIST]
```

**Parameters**:
- `--project-folder=PATH`: Optional - Specific folder to analyze (default: entire project root)
- `--output-folder=PATH`: Optional - Custom location to save reports (default: `/docs/agents`)
- `--ignore-folders=LIST`: Optional - Comma-separated list of folders/files to exclude from analysis

**Examples**:
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

**Workflow**:
1. **Phase 1**: Initialize project structure and MANIFEST.md
2. **Phase 2**: Execute dependency audit and architectural analysis in parallel
3. **Phase 3**: Execute component deep-dive analysis in parallel for all identified components
4. **Phase 4**: Validate completeness and finalize MANIFEST
5. **Phase 5**: Generate project overview summary
6. **Phase 6**: Generate README index

**Output Structure**:
```
{output-folder}/
├── MANIFEST.md                                    # Execution registry
├── PROJECT-OVERVIEW-{timestamp}.md              # Executive summary
├── README-{timestamp}.md                         # Index of all reports
├── dependency-auditor/
│   └── dependencies-report-{timestamp}.md
├── architectural-analyzer/
│   └── architectural-report-{timestamp}.md
└── component-deep-analyzer/
    ├── component-analysis-{Component1}-{timestamp}.md
    ├── component-analysis-{Component2}-{timestamp}.md
    └── ...
```

**Reports Generated**:
- **Dependency Audit Report**: Outdated dependencies, vulnerabilities, license compatibility
- **Architectural Report**: System architecture, components, integration points, risks
- **Component Analysis Reports**: Deep-dive into each identified component (business rules, dependencies, endpoints, test coverage)

---

## Agents Overview

The Project Analyzer plugin includes three specialized agents:

### 1. Architectural Analyzer

**Purpose**: Comprehensive architectural analysis of the codebase

**Capabilities**:
- Maps complete system architecture and component relationships
- Identifies critical components and coupling patterns
- Analyzes afferent and efferent coupling
- Documents integration points with external systems
- Evaluates architectural risks and single points of failure
- Assesses infrastructure patterns and deployment architecture
- Identifies architectural debt and security risks

**Output**: Architectural Analysis Report with:
- Executive Summary
- System Overview
- Critical Components Analysis (table with coupling metrics)
- Dependency Mapping
- Integration Points
- Architectural Risks & Single Points of Failure
- Technology Stack Assessment
- Security Architecture and Risks
- Infrastructure Analysis (if present)

### 2. Component Deep Analyzer

**Purpose**: Deep technical analysis of individual software components

**Capabilities**:
- Maps complete internal structure and organization
- Extracts and documents all business rules and validation logic
- Analyzes implementation details and algorithms
- Identifies all dependencies (internal and external)
- Documents design patterns and architectural decisions
- Evaluates component coupling, cohesion, and boundaries
- Assesses security measures and error handling
- Identifies technical debt and code smells
- Analyzes test coverage

**Output**: Component Deep Analysis Report with:
- Executive Summary
- Data Flow Analysis
- Business Rules & Logic (overview table + detailed breakdown)
- Component Structure
- Dependency Analysis
- Afferent and Efferent Coupling
- Endpoints (REST, GraphQL, gRPC, etc.)
- Integration Points
- Design Patterns & Architecture
- Technical Debt & Risks
- Test Coverage Analysis

**Usage**: Invoked automatically by `/generate-architectural-report` for each component identified in the architectural analysis.

## Workflow

### Full Architectural Report Workflow

```
Phase 1: Initialize
  ├── Create folder structure
  └── Create MANIFEST.md

Phase 2: Parallel Analysis
  ├── Dependency Audit (parallel)
  └── Architectural Analysis (parallel)

Phase 3: Component Deep-Dive
  └── For each component (parallel):
      └── Component Deep Analysis

Phase 4: Validation
  └── Verify completeness and finalize MANIFEST

Phase 5: Synthesis
  ├── Generate Project Overview
  └── Generate README Index
```

## Usage Examples

### Complete Architectural Analysis

```bash
# Full analysis of entire project
/generate-architectural-report

# Analyze specific folder
/generate-architectural-report --project-folder=src/services

# Custom output with exclusions
/generate-architectural-report \
  --project-folder=src \
  --output-folder=reports/architecture \
  --ignore-folders=node_modules,dist,test,.git
```

### Incremental Analysis

```bash
# Step 1: Dependency audit
/run-dependency-audit --output-folder=reports

# Step 2: Full architectural analysis (uses dependency report)
/generate-architectural-report --output-folder=reports
```

---

## Output Structure

### Full Architectural Report Output

```
docs/agents/                                      # Default output
├── MANIFEST.md                                   # Execution registry
├── PROJECT-OVERVIEW-{timestamp}.md              # Executive summary
├── README-{timestamp}.md                        # Index
├── dependency-auditor/
│   └── dependencies-report-{timestamp}.md
├── architectural-analyzer/
│   └── architectural-report-{timestamp}.md
└── component-deep-analyzer/
    ├── component-analysis-PaymentService-{timestamp}.md
    ├── component-analysis-AuthService-{timestamp}.md
    └── ...
```

## Important Notes

### Analysis Scope

- **Read-Only**: All agents are read-only and never modify project files
- **Direct Dependencies Only**: Dependency auditor catalogs only direct dependencies (ignores transitives)
- **Architecturally Significant**: Focuses on architecturally significant components, not every file
- **Parallel Processing**: Multiple components analyzed in parallel for faster results

### Report Characteristics

- **Timestamped Files**: All reports include timestamps in filenames
- **MANIFEST Tracking**: Full report workflow tracks execution state in MANIFEST.md
- **Comprehensive**: Reports include all relevant sections with detailed analysis
- **Actionable**: Reports provide specific findings with file paths and line numbers

### Prerequisites

- **Source Code Access**: Requires read access to project source code
- **Package Files**: Dependency audit requires package management files (package.json, requirements.txt, etc.)
- **Git Repository**: Some analysis benefits from git history (optional)

### Limitations

- **No Code Modification**: Agents never modify or refactor code
- **No Recommendations**: Reports focus on findings, not suggestions
- **No Time Estimates**: Reports don't include time estimates for fixes
- **Analysis Only**: Provides insights, not implementation guidance

## Troubleshooting

| Issue | Solution |
|-------|----------|
| No source code found | Provide correct project path or confirm analysis scope |
| No dependency files found | Ensure package management files exist (package.json, requirements.txt, etc.) |
| Component not found | Check component name spelling or run architectural analysis first |
| Permission denied | Check file read permissions |
| Analysis timeout | Use `--project-folder` to analyze smaller scope |
| MANIFEST not found | Run `/generate-architectural-report` to create MANIFEST |

## Integration with Other Plugins

The Project Analyzer plugin works well with:

- **ADRs Management**: Use architectural reports to identify decisions worth documenting
- **Diagrams Generator**: Generate diagrams from architectural findings

Example workflow:
```bash
# 1. Analyze architecture
/generate-architectural-report

# 2. Document decisions
/adr-map
/adr-identify AUTH DATA API

# 3. Generate diagrams for key components
/c4-generate docs/features/payment-fdd.md
```

