# Generate Development Guideline

You are tasked with creating a comprehensive development guideline document for **{{LANGUAGE}}** following the template structure provided at the end of this file.

## COMMAND SYNTAX

```
/generate-development-guideline <language> [--param=value ...]
```

**Supported Parameters** (all optional):
- `--orm=<name>` - ORM or query builder (e.g., prisma, sqlalchemy, sqlc, gorm, hibernate)
- `--web=<name>` - Web framework (e.g., express, fastapi, chi, spring-boot, gin, flask)
- `--framework=<name>` - Main framework (e.g., laravel, nestjs, langgraph, langchain, spring, django)
- `--db=<name>` - Database driver (e.g., pgx, asyncpg, mysql2, jdbc, psycopg2)
- `--testing=<name>` - Testing framework (e.g., jest, pytest, testify, junit, vitest)
- `--logging=<name>` - Logging library (e.g., winston, structlog, zap, logrus, log4j)
- `--validation=<name>` - Validation library (e.g., zod, pydantic, validator, joi)
- `--http=<name>` - HTTP client (e.g., axios, requests, resty, okhttp, httpx)
- `--di=<name>` - Dependency injection (e.g., inversify, wire, spring, dagger)
- `--async=<name>` - Async runtime (e.g., tokio, asyncio, async-std, gevent)
- `--serialization=<name>` - Serialization library (e.g., serde, jackson, gson, msgpack)

**Examples**:
```
/generate-development-guideline Go --orm=sqlc --web=chi --db=pgx --testing=testify
/generate-development-guideline TypeScript --orm=prisma --web=express --testing=jest --validation=zod
/generate-development-guideline Python --orm=sqlalchemy --web=fastapi --logging=structlog
/generate-development-guideline Rust --orm=diesel --web=axum --async=tokio --serialization=serde
```

## QUALITY REQUIREMENTS

### Document Target
- **Length**: 1000-1500 lines total
- **Code blocks**: Minimum 20
- **Good vs Bad examples**: Minimum 5
- **Commands**: Minimum 15 executable examples
- NO emojis, NO placeholders (TODO, TBD)

### Writing Style
- **CONCISE**: Quick reference, not a book
- **PRACTICAL**: Show code, not just theory
- **FOCUSED**: Cover essentials, skip edge cases

### Critical Sections
Must have code examples: 7 (Functions), 8 (Errors), 11 (Tests), 22 (Database), 23 (Logs)

## CRITICAL REQUIREMENTS

1. **Follow the template structure**: Read and strictly adhere to the template structure provided at the end of this file
2. **Correct section numbering**: Sections MUST be numbered sequentially (1, 2, 3, ..., N) with NO gaps
3. **Optional sections**: Evaluate each `[OPCIONAL]` section and include ONLY if the language supports that feature
4. **When skipping sections**: Adjust ALL subsequent numbering to maintain sequential order
5. **Research-backed**: ALL content must be based on authoritative sources found through web research

## EXECUTION PROCESS

### Phase 0: Parameter Parsing and Auto-Defaults (FIRST STEP)

**Parse the command invocation** to extract library/framework preferences from the parameters.

**Create a "Project Stack" configuration with auto-defaults**:

1. Extract all `--key=value` parameters from the command
2. **Auto-populate essential categories** if not specified:
   - `testing`: Auto-select most popular framework for the language
   - `formatting`: Auto-select standard formatter (black, gofmt, prettier, rustfmt)
   - `linting`: Auto-select standard linter for the language
   - `logging`: Auto-select stdlib logging if good, or most popular library
   - `build_tool`: Auto-select native build tool when applicable (make, cargo, gradle)
3. **Do NOT auto-populate opinionated categories** (orm, web framework, db driver)
4. Build final stack object with specified + auto-populated libraries

**Auto-Default Rules by Language**:

**Go**:
```yaml
testing: "testing (stdlib) + testify for assertions"
formatting: "gofmt, goimports"
linting: "go vet, staticcheck, golangci-lint"
logging: "log/slog (Go 1.21+)"
build_tool: "go build, make"
```

**Python**:
```yaml
testing: "pytest (most popular) or unittest (stdlib)"
formatting: "black"
linting: "flake8, pylint"
type_checking: "mypy"
logging: "logging (stdlib)"
build_tool: "pip, poetry"
```

**TypeScript/JavaScript**:
```yaml
testing: "jest or vitest"
formatting: "prettier"
linting: "eslint"
type_checking: "tsc"
logging: "winston or pino"
build_tool: "npm, pnpm, yarn"
```

**Rust**:
```yaml
testing: "cargo test (stdlib)"
formatting: "rustfmt"
linting: "clippy"
logging: "tracing or log crate"
build_tool: "cargo"
```

**Example parsing with auto-defaults**:
```
Command: /generate-development-guideline Go --orm=sqlc --web=chi --db=pgx

Result:
{
  "language": "Go",
  "stack": {
    "orm": "sqlc",              // User specified
    "web": "chi",               // User specified
    "db": "pgx",                // User specified
    "testing": "testify",       // AUTO-POPULATED
    "logging": "log/slog",      // AUTO-POPULATED
    "formatting": "gofmt",      // AUTO-POPULATED
    "linting": "staticcheck",   // AUTO-POPULATED
    "build_tool": "make",       // AUTO-POPULATED
    "validation": null,         // Not applicable for Go
    "http": null,               // Not specified, not auto-populated
    "di": null,                 // Not specified, not auto-populated
    "async": null,              // N/A for Go (no async runtime)
    "serialization": null       // Not specified, not auto-populated
  }
}
```

**Output Phase 0 Report**:
```
PROJECT STACK CONFIGURATION
Language: {{LANGUAGE}}

User-Specified Libraries:
  - ORM: {{orm}}
  - Web Framework: {{web}}
  - Database Driver: {{db}}

Auto-Populated Essential Tools:
  - Testing: {{testing}} (auto-selected)
  - Formatting: {{formatting}} (auto-selected)
  - Linting: {{linting}} (auto-selected)
  - Logging: {{logging}} (auto-selected)
  - Build Tool: {{build_tool}} (auto-selected)

Not Specified (will use language-generic guidance):
  - HTTP Client
  - Dependency Injection
  - [... other categories]

NOTE: Auto-populated tools are language standards that every project should use.
User-specified libraries will be listed in Project Stack section for reference only.
All code examples will use stdlib/language-native features only.
```

### Phase 1: Research (MANDATORY)

Use **WebSearch** extensively to find and analyze. **Minimum 5 official sources required.**

**1. Official Documentation** (MANDATORY - minimum 3 sources):
- Official language documentation and tutorials
- Official style guides and coding standards
- Official best practices documentation
- Language specification documents

**2. Authoritative Industry Guidelines** (minimum 2 sources):
- Style guides from major companies (Google, Uber, Airbnb, Microsoft, Meta, Netflix, etc.)
- Guidelines from prominent open-source projects
- Well-established community standards and conventions
- Academic or institutional coding standards

**3. Essential Ecosystem Tools** (research for ALL languages):
- Official package/dependency managers
- Standard code formatters and linters
- Native or most popular testing frameworks
- Build tools and task runners (when applicable)
- Profiling and debugging tools

**4. Language Characteristics** (deep analysis required):
- Type system: static/dynamic, strong/weak typing
- Concurrency model: threads, async/await, goroutines, actors, etc.
- Interface/abstraction mechanisms: interfaces, traits, protocols, duck typing
- Memory management: garbage collection, manual, reference counting, ownership
- Profiling and benchmarking capabilities
- Standard database connectivity approaches (drivers, connection patterns)
- Native logging capabilities and standard approaches

**5. Real-World Examples** (find at least 3 production codebases):
- Production-grade project structures
- Industry-standard naming conventions
- Common patterns and anti-patterns with examples

**6. Library Research** (for user-specified AND auto-populated):

For **ALL libraries in Project Stack** (user-specified + auto-populated):
- Official documentation URL or GitHub repository URL
- Brief description of purpose (1-2 sentences max)
- Latest stable version number
- **ONLY metadata** - do NOT research usage patterns or code examples

**Research Completeness Check**:
- [ ] Minimum 5 official sources documented
- [ ] At least 3 production codebase examples found
- [ ] All essential tools identified (formatter, linter, testing, build)
- [ ] Language characteristics fully analyzed
- [ ] All Project Stack libraries have: name, version, purpose, link

**Document ALL sources** with URLs for the References section.

### Phase 2: Analysis and Planning

Based on your research findings, determine:

**1. Which `[OPCIONAL]` sections to include**:

Evaluate each optional section:

- **Section 4 (Docker)**: Include if containerized development is commonly practiced in the language ecosystem. Always use the official and smallest available Docker image, preferably an Alpine variant, and pin to the latest stable version. Never use multi-stage builds for development environments.
- **Section 6 (Types)**: Include if language has static typing or strong type system
- **Section 9 (Concurrency)**: Include if language has native concurrency primitives
- **Section 10 (Interfaces)**: Include if language has interfaces, traits, protocols, or similar abstractions
- **Section 12 (Mocks)**: Include if mature mocking tools or patterns exist
- **Section 14 (Load Testing)**: Include if language-specific load testing tools or practices are established
- **Section 15 (Profiling)**: Include if mature profiling tools are available
- **Section 16 (Benchmarks)**: Include if native or well-established benchmarking exists
- **Section 17 (Optimization)**: Include if language-specific optimization techniques are documented

**2. Calculate final section count**:
- Count all mandatory sections (those without `[OPCIONAL]`)
- Add included optional sections
- Total = final number of sections in your document

**3. Create numbering mapping**:
- Map each template section to its final document section number
- Ensure sequential numbering with NO gaps
- Example: If section 9 is excluded, section 10 becomes section 9, section 11 becomes 10, etc.

**4. Document your decisions**:
- List which optional sections you're including
- Explain WHY each was included or excluded based on research

**5. Finalize Project Stack**:
- **If ANY parameter was specified**: prepare minimal metadata (name, purpose, link, version)
- **If NO parameters specified**: Project Stack section will NOT be included in document
- Verification that specified libraries (if any) are appropriate for {{LANGUAGE}}

### Phase 3: Multi-Phase Content Generation

**CRITICAL**: Document generation is split into 4 sub-phases to stay under 32K token output limit.

**CONCISENESS RULES** (apply to ALL sub-phases):
- Each section: 30-50 lines maximum
- Maximum 3 subsections per section
- One code example per concept
- Zero long explanations, focus on code and commands
- Direct and practical, not verbose

**Global Generation Rules** (apply to ALL sub-phases):

**1. Numbering**:
- Use ONLY sequential numbers: 1, 2, 3, 4, ..., N
- NO gaps in numbering
- Remove ALL `[OPTIONAL]` markers from section titles
- If a section is optional but included, it's just a regular section

**2. Content Quality**:
- Fill each section with substantive, practical content
- Use REAL code examples with proper syntax
- Include actual tool names, package names, and versions
- Provide working commands and configurations
- Reference authoritative sources with links

**3. Formatting Consistency**:
- Use proper markdown formatting
- Code blocks with correct language syntax highlighting
- Tables for command references and comparisons
- Bullet points for lists
- Consistent heading levels

**4. Language-Specific Authenticity**:
- Use idiomatic code for the language
- Follow naming conventions discovered in research
- Include language-specific best practices
- Use only standard library or language-native features
- Use realistic project structure examples for the language

**5. Completeness**:
- Every section must have complete content (NO placeholders like "TODO" or "Add content here")
- Checklist section must include all relevant validation items
- References section must list all major sources consulted
- Docker section (if included) must have complete setup instructions

**6. Library Integration Strategy** (CRITICAL - Moderate Approach):

**Code Examples Philosophy**:
- **ALL code examples** must use **standard library or language-native features ONLY**
- Examples demonstrate language patterns, NOT framework-specific implementations
- Goal: Guideline is useful regardless of which libraries developer chooses

**Example Approach by Section**:
- **Database (Section 22)**: Use stdlib database driver (database/sql for Go, sqlite3 for Python)
- **Testing (Section 11)**: Use native testing framework or most basic approach
- **Logging (Section 23)**: Use stdlib logging (log/slog, logging module)
- **Web/HTTP**: Generic HTTP server patterns using stdlib (http.Server, http.server)
- **Docker**: Only install runtime dependencies, not application libraries

**Project Stack Section** (reference only):

**ALWAYS include "Project Stack" section** when ANY libraries are in the stack (user-specified OR auto-populated):

```markdown
## Project Stack

The following libraries were specified for reference in this project:

**User-Specified Libraries**:
- **ORM/Database**: {{orm}} (v{{version}}) - {{purpose}} - {{link}}
- **Web Framework**: {{web}} (v{{version}}) - {{purpose}} - {{link}}

**Auto-Populated Essential Tools**:
- **Testing**: {{testing}} (v{{version}}) - {{purpose}} - {{link}}
- **Formatting**: {{formatting}} - {{purpose}} - {{link}}
- **Linting**: {{linting}} - {{purpose}} - {{link}}
- **Logging**: {{logging}} (v{{version}}) - {{purpose}} - {{link}}

> **Note**: This section lists libraries for quick reference.
> All code examples in this guideline use standard library or language-native features.
> Principles and patterns apply regardless of library choices.
```

**Position**: Place Project Stack section immediately after title, before Section 1.

**If Project Stack is empty** (no user params, language has no auto-defaults): DO NOT include section.

---

### Phase 3.1: Generate Foundation (Sections 1-8)

Generate sections 1-8 (Core Principles through Error Handling).

**Content**:
- Document title + Project Stack (if applicable)
- Sections 1-8 with code examples in sections 7 and 8

**Line Limit**: Maximum 400 lines total for this phase

**Write to**: `{{LANGUAGE}}-development-guidelines.md`

---

### Phase 3.2: Generate Core Implementation (Sections 9-16)

Generate sections 9-16 (Concurrency through Benchmarks).

**Content**:
- Sections 9-16 with code examples in section 11 (Tests)

**Line Limit**: Maximum 350 lines total for this phase

**Append to**: `{{LANGUAGE}}-development-guidelines.md`

---

### Phase 3.3: Generate Practices & Patterns (Sections 17-21)

Generate sections 17-21 (Optimization through Comments).

**Content**:
- Sections 17-21 with practical examples

**Line Limit**: Maximum 250 lines total for this phase

**Append to**: `{{LANGUAGE}}-development-guidelines.md`

---

### Phase 3.4: Generate Database, Logs & Finalization (Sections 22-26)

Generate sections 22-26 (Database through References).

**Content**:
- Sections 22-23 with code examples (Database, Logs)
- Sections 24-26 (Golden Rules, Checklist, References)

**Line Limit**: Maximum 350 lines total for this phase

**Append to**: `{{LANGUAGE}}-development-guidelines.md`

### Phase 3.5: Quick Validation

Count total lines in the document.

**If document has MORE than 1500 lines**:
1. Remove verbose explanations and long introductions
2. Consolidate similar examples into one
3. Keep all code examples and commands
4. Reduce subsections to maximum 3 per section
5. Check line count again and repeat until document is 1000-1500 lines

**Also verify**:
- Critical sections have code examples (7, 8, 11, 22, 23)
- Minimum requirements met (20 code blocks, 5 good vs bad, 15 commands)

**If any requirement fails**: Fix before proceeding to Phase 4.

### Phase 4: Final Validation

Quick final checks before delivery:

**Final Checks**:
- [ ] Sections numbered sequentially (no gaps)
- [ ] NO `[OPTIONAL]` markers remain
- [ ] Document is 1000-1500 lines
- [ ] All requirements met (see QUALITY REQUIREMENTS section)

**If Project Stack used**:
- [ ] "Project Stack" section exists after title
- [ ] Code examples use ONLY stdlib (not user libraries)

## OUTPUT DELIVERABLES

Provide the following:

**1. Phase 0 Output** (Parameter Parsing):
- Project Stack configuration showing only specified libraries
- List of parameters provided by user
- Note if NO parameters were provided (guideline will be 100% generic)

**2. Research Summary** (brief, 5-10 lines):
- Key authoritative sources consulted
- Major style guides/standards identified
- Notable characteristics of the language discovered
- Language ecosystem tools (package managers, formatters, linters)

**3. Section Inclusion Report**:
- List all optional sections
- For each: "INCLUDED" or "EXCLUDED" with brief reason
- Example: "Section 9 (Concurrency): INCLUDED - Language has native async/await"

**4. Project Stack Final Report** (ONLY if parameters provided):
```
PROJECT STACK (FINAL)
- ORM: <name> v<version> - <purpose> - <link>
- Web: <name> v<version> - <purpose> - <link>
- Database: <name> v<version> - <purpose> - <link>
- Testing: <name> v<version> - <purpose> - <link>
- Logging: <name> v<version> - <purpose> - <link>
- [only categories that were specified via parameters]
```

**If NO parameters provided**: Report "No libraries specified - guideline will be 100% language-generic"

**5. Final Document**:
- Complete guideline document
- **Include "Project Stack" section ONLY if parameters were provided**
- Save as: `{{LANGUAGE}}-development-guidelines.md`
- Place in current working directory

**6. Validation Report**:
- Confirm all validation checklist items passed
- Note any special considerations or deviations
- Total section count in final document
- Confirmation that guideline is 100% language-generic (not framework-specific)
- Confirmation that code examples use only standard library or language-native features

## ANTI-PATTERNS TO AVOID

**DO NOT**:
- Leave gaps in section numbering (e.g., 1, 2, 4, 5 - missing 3)
- Keep `[OPCIONAL]` markers in final document
- Use generic placeholder content or TODO markers
- Include sections not supported by the language
- Provide inaccurate tool names or commands
- Copy-paste from other languages without adaptation
- Skip the validation phase
- Deliver incomplete sections
- Use outdated or deprecated practices
- Use specified libraries in code examples (examples must use stdlib only)
- Create framework-specific tutorials (guideline must be language-focused)
- Include "Project Stack" section when stack is empty

**DO**:
- Renumber ALL sections when excluding optionals
- Remove `[OPCIONAL]` markers from included sections
- Fill every section with practical content (30-50 lines per section)
- Research thoroughly before writing (minimum 5 official sources)
- Provide working code examples using standard library only
- Follow the language's idiomatic style
- Include "Project Stack" section when ANY libraries in stack (user + auto-populated)


## EXAMPLE: Section Numbering

**Scenario**: Creating guidelines for a language that has:
- Static typing → Include section 6
- NO native concurrency → Exclude section 9
- Has interfaces → Include section 10
- Docker commonly used → Include section 4

**Template → Final mapping**:
```
Template                          Final Document
1. Principles                  →  1. Principles
2. Project Init                →  2. Project Init
3. Structure                   →  3. Structure
4. [OPCIONAL] Docker           →  4. Docker (INCLUDED)
5. Nomenclature                →  5. Nomenclature
6. [OPCIONAL] Types            →  6. Types (INCLUDED)
7. Functions                   →  7. Functions
8. Errors                      →  8. Errors
9. [OPCIONAL] Concurrency      →  (EXCLUDED)
10. [OPCIONAL] Interfaces      →  9. Interfaces (INCLUDED, renumbered)
11. Tests                      →  10. Tests (renumbered)
12. [OPCIONAL] Mocks           →  (EXCLUDED)
13. Integration Tests          →  11. Integration Tests (renumbered)
...                            →  ...continue renumbering
```

**Key point**: Sequential numbers with NO gaps. When you exclude section 9, section 10 becomes section 9.

## EXAMPLE: Library Treatment

**WRONG** (Library-specific content in main sections):
```markdown
## 22. Database

### 22.1 Using Prisma ORM
npm install prisma @prisma/client

### 22.2 Prisma Schema
[Prisma-specific examples and patterns]
```

**CORRECT** (Language-generic content):
```markdown
## 22. Database

### 22.1 Abordagem
TypeScript supports ORMs, query builders, and raw SQL approaches.

### 22.2 Conexão
Example using standard PostgreSQL driver (pg):
[Generic TypeScript database connection example]

### 22.3 Boas Práticas
[Language-level database best practices]
```

**Project Stack section** (ONLY if --orm=prisma was specified):
```markdown
## Project Stack

- **ORM**: Prisma (v5.8.0) - Type-safe database ORM for Node.js and TypeScript - https://www.prisma.io
```

## READY TO START

**FIRST**: Parse command parameters to extract Project Stack configuration (Phase 0).

**THEN**: Read the template structure at the end of this file.

**FINALLY**: Begin Phase 1 research for **{{LANGUAGE}}** and specified libraries.

---

# TEMPLATE STRUCTURE

The following template defines the structure and sections for language development guidelines.

> **Note on Optional Sections:**
> This template contains sections marked as **[OPTIONAL]** that should be included or excluded according to the language's characteristics and peculiarities. Not all languages have the same features:
> - Some languages don't have strong static typing (Types section)
> - Some don't have native concurrency primitives (Concurrency section)
> - Some don't have support for interfaces or similar abstractions (Interfaces section)
> - Profiling and benchmarking tools may not be available or mature
> - Docker may not be applicable for all languages
>
> **Instruction:** When creating guidelines for a specific language, evaluate each [OPTIONAL] section and include only those that make sense for the language in question.

## 1. Core Principles

### 1.1 Philosophy and Style
- Use automatic formatting tools
- Follow language idiomatic conventions
- Clear and simple code beats complex abstractions
- Use validation and linting tools

### 1.2 Clarity over Brevity
- Names should communicate intent
- Self-explanatory code reduces need for comments
- Avoid premature optimization: clarity first, performance later

## 2. Project Initialization

### 2.1 Creating New Project
Commands to initialize project with package manager and configure namespace/repository.

### 2.2 Dependency Management
Commands to add, remove, and update dependencies.

## 3. Project Structure

Standard directory layout for the language, including:
- Main source code
- Tests
- Configuration
- Documentation
- Build output
- Dependency files

## [OPTIONAL] 4. Container Development (Docker)

**Include if the language and ecosystem benefit from containerized development.**

### 4.1 Container Philosophy
Every development project should use Docker when available to ensure:
- Consistent environment across developers
- No runtime dependencies installed locally
- Same version in development and production
- Dependency isolation

### 4.2 Docker File Structure
List of necessary Docker files (Dockerfile, docker-compose.yaml, .dockerignore).

### 4.3 Dockerfile for Development
Dockerfile focused on development.
Use `sleep infinity` to keep container running.

### 4.4 Docker Compose
Configuration with:
- Application service
- Dependencies (database, cache, etc.)
- Volumes for hot reload and cache
- Networks
- Healthchecks

### 4.5 .dockerignore
Essential list of files/directories to ignore in build.

### 4.6 Essential Commands
Table with main commands:
- Start environment
- View logs
- Execute application
- Run tests
- Interactive shell
- Stop environment

### [OPTIONAL] 4.7 Makefile (only if language has build and deployment automation tools. e.g., Go. Do not include for languages like TypeScript, PHP, etc.)
Simplified commands for common operations.

### 4.8 Best Practices
Specific recommendations for using Docker with the language.

## 5. Naming Conventions

Language naming conventions for:
- Packages/Modules
- Classes/Structs/Types
- Functions/Methods
- Variables
- Constants
- Files

## [OPTIONAL] 6. Types and Type System

**Include only if language has strong static typing.**

### 6.1 Type Declaration
How to declare types, structs, classes, enums.

### 6.2 Type Safety
Practices to ensure type safety.

### 6.3 Allocation and Initialization
How to allocate and initialize data structures.

## 7. Functions and Methods

### 7.1 Signatures
Patterns for declaring functions with types, parameters, and returns.

**MUST include code example** showing:
- Function declaration with parameters
- Return type annotation (if applicable)
- Error handling in signature

### 7.2 Returns and Errors
How to return values and handle errors idiomatically.

**MUST include "Good vs Bad" example** showing:
- Good: Proper error handling and return patterns
- Bad: Error swallowing or unclear returns

### 7.3 Best Practices
- Functions with single responsibility
- Parameter limit (prefer 3-4, use objects for more)
- Avoid hidden side effects
- Document pre/post-conditions when relevant

## 8. Error Handling

### 8.1 Philosophy
Language's error handling model (exceptions, error values, result types, etc.).

**MUST include code example** showing:
- How errors are created and raised/returned
- Error wrapping with context
- Custom error types for domain logic

### 8.2 Conventions
How to handle, propagate, and log errors idiomatically.

**MUST include "Good vs Bad" example** showing:
- Good: Explicit error handling with context
- Bad: Silent error swallowing or generic error messages

### 8.3 Best Practices
- Never ignore errors silently
- Add useful context (IDs, values, operation)
- Custom errors for application domain
- Log errors at I/O boundaries, not in every layer

## [OPTIONAL] 9. Concurrency and Parallelism

**Include if language has native concurrency primitives (goroutines, async/await, threads, etc.).**

### 9.1 Concurrency Model
Threads, async/await, goroutines, or specific model.

### 9.2 Synchronization
Mutexes, locks, channels, promises.

### 9.3 Best Practices
- Control lifecycle
- Avoid race conditions
- Use timeouts
- Graceful shutdown

### 9.4 Common Pitfalls
Common concurrency traps in the language.

## [OPTIONAL] 10. Interfaces and Abstractions

**Include if language has interfaces, traits, protocols, or similar abstraction mechanisms.**

### 10.1 Interface Design
How to design small, cohesive interfaces.

### 10.2 Implementation
How to implement and validate implementations.

### 10.3 Composition
How to compose interfaces to create larger abstractions.

## 11. Unit Tests

### 11.1 Structure
Framework and test writing patterns.

**MUST include code example** showing:
- Basic test structure and organization
- Assertions and validation patterns
- Test naming conventions

### 11.2 Table-Driven Tests
Parametrized tests when applicable in the language.

**MUST include code example** showing:
- Parametrized or table-driven test structure
- Multiple test cases in compact format

### 11.3 Assertions
How to make assertions and validations.

### 11.4 Commands
Commands to run tests with coverage, specific tests.

**MUST include executable commands** showing:
- Run all tests
- Run specific test
- Run with coverage
- Run with verbosity/detailed output

## [OPTIONAL] 12. Mocks and Testability

**Include if language has mature mocking tools or established patterns.**

### 12.1 Mock Strategies
Manual mocks vs mocking libraries.

### 12.2 Dependency Injection
How to structure code for testability.

### 12.3 Test Doubles
Mocks, stubs, fakes, spies.

## 13. Integration Tests

### 13.1 Structure and Organization
Build tags, markers, or mechanisms to separate integration tests.

### 13.2 Selective Execution
How to run only unit tests or only integration tests.

### 13.3 Real Dependencies
Use of testcontainers or similar tools when applicable.

## [OPTIONAL] 14. Load and Stress Tests

**Include if there are language-specific tools or established practices for load testing.**

### 14.1 Tools
Available load testing tools.

### 14.2 Load Benchmarks
How to write benchmarks that simulate real load.

### 14.3 Concurrency Tests
How to test behavior under concurrent access.

## [OPTIONAL] 15. Profiling and Diagnostics

**Include if language has mature profiling tools.**

### 15.1 CPU and Memory Profiling
Tools for performance analysis.

### 15.2 Diagnostic Tools
Profilers, debuggers, memory inspectors.

### 15.3 Performance Analysis
How to capture and analyze profiles.

## [OPTIONAL] 16. Benchmarks

**Include if language has native or established benchmarking framework.**

### 16.1 Writing Benchmarks
How to write valid benchmarks.

### 16.2 Sub-benchmarks
Parametrized benchmarks.

### 16.3 Execution and Analysis
Commands to run and compare benchmarks.

## [OPTIONAL] 17. Optimization

**Include relevant language-specific optimization techniques.**

### 17.1 Principles
Measure first, low-hanging fruit, document trade-offs.

### 17.2 Common Optimizations
Pre-allocation, caching, object reuse, lazy loading.

### 17.3 Memory Optimization
Specific techniques to reduce allocations and memory consumption.

### 17.4 Basic Performance
General performance best practices (conversions, concatenations, etc.).

## 18. Security

### 18.1 Essential Practices
- Never hardcode secrets
- Validate external input
- HTTPS in communications
- Rate limiting
- Updated dependencies
- Principle of least privilege

### 18.2 Tools
Specific vulnerability scanners.

### 18.3 Security at API Boundaries
Defensive copying, input validation, sanitization.

## 19. Code Patterns

### 19.1 Early Return
Reducing nesting.

### 19.2 Separation of Concerns
Logic separated from I/O.

### 19.3 DRY
Extract duplication, but avoid premature abstraction.

### 19.4 Variable Scope
Minimize variable scope.

## 20. Dependency Management

### 20.1 Principles
- Standard library first
- Well-maintained dependencies
- Minimalism
- Explicit versioning

### 20.2 Commands
Check vulnerabilities, update, clean.

## 21. Comments and Documentation

### 21.1 Code Comments
Comment "why", not "what".

### 21.2 API Documentation
Language's documentation style (docstrings, JSDoc, GoDoc, etc.).

### 21.3 Package Documentation
How to document packages/modules.

## 22. Database

### 22.1 Approach
Available approaches in the language: ORM, Query Builder, or Raw SQL.
Discussion of trade-offs for each approach.

### 22.2 Connection and Driver
How to connect to databases using standard driver or native language library.

**CRITICAL: Use ONLY standard library or official language drivers in examples.**

**MUST include code example** showing:
- Database connection using stdlib/official driver
- Connection configuration (pool size, timeouts, etc.)
- Proper connection cleanup/closing

**MUST include code example** showing:
- Query execution with prepared statements/parameterized queries
- Safe parameter binding (prevent SQL injection)
- Result handling and iteration

### 22.3 Migrations
Migration concept and how they are managed in the language.
Common patterns and approaches in the language ecosystem.

### 22.4 Best Practices
- Prepared statements / parametrized queries (NEVER string concatenation)
- Appropriate indexes for frequent queries
- Connection pooling for reuse
- Explicit transaction management
- Connection error handling (timeouts, retry logic)

## 23. Logs and Observability

### 23.1 Log Levels
DEBUG, INFO, WARN, ERROR, FATAL.
How the language categorizes log severity.

### 23.2 Structured Logs
Concept of structured logs (JSON, key-value).
How to implement using native language features.

**MUST include code example** showing:
- Logger setup and configuration
- Log level configuration
- Output destination (stdout, file, etc.)

### 23.3 Logging Implementation
How to implement logging using standard library or native features.
Practical examples without external dependencies.

**MUST include code example** showing:
- Structured logging with context fields
- Including request IDs, user IDs, or correlation data
- Practical usage in application code

### 23.4 Metrics and Observability
Metrics collection: latency, error rate, throughput, resources.
Code instrumentation principles.
- Instrument I/O operations and system boundaries
- Expose metrics via endpoints (health, readiness, metrics)
- Keep label cardinality controlled

## 24. Golden Rules

Universal principles adapted for the language:
1. Simplicity
2. Explicit errors
3. Tests
4. Documentation
5. Measured performance

## 25. Pre-Commit Checklist

### Code
- [ ] Formatting applied
- [ ] Linter without critical errors
- [ ] Code compiles/runs without errors

### Tests
- [ ] All tests pass
- [ ] Coverage >= 70% on critical code
- [ ] Integration tests executed (if applicable)
- [ ] Benchmarks validated (if there are changes)

### Quality
- [ ] Errors handled explicitly
- [ ] Resources with proper cleanup
- [ ] No hardcoded secrets
- [ ] Dependencies without vulnerabilities

### Documentation
- [ ] Public functions/classes documented
- [ ] README updated
- [ ] Comments explain "why"

### Docker (if applicable)
- [ ] Valid Dockerfile
- [ ] Functional docker-compose.yaml
- [ ] Application starts without errors in container

## 26. References

### Official Documentation
Links to language documentation, best practices, style guides.

### Essential Tools
Links to package manager, formatter, linter, testing framework.

### Testing and Performance
Links to profiling tools, load testing, testcontainers.

### Community
Forum, resource lists, awesome lists.
