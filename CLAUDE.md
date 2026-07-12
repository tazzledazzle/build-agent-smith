# Project Instructions for AI Agents

This file provides instructions and context for AI coding agents working on this project.

<!-- BEGIN BEADS INTEGRATION v:1 profile:minimal hash:6cd5cc61 -->

## Beads Issue Tracker

This project uses **bd (beads)** for issue tracking. Run `bd prime` to see full workflow context and commands.

### Quick Reference

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --claim  # Claim work
bd close <id>         # Complete work
```

### Rules

- Use `bd` for ALL task tracking — do NOT use TodoWrite, TaskCreate, or markdown TODO lists
- Run `bd prime` for detailed command reference and session close protocol
- Use `bd remember` for persistent knowledge — do NOT use MEMORY.md files

**Architecture in one line:** issues live in a local Dolt DB; sync uses `refs/dolt/data` on your git remote; `.beads/issues.jsonl` is a passive export. See https://github.com/gastownhall/beads/blob/main/docs/SYNC_CONCEPTS.md for details and anti-patterns.

## Agent Context Profiles

The managed Beads block is task-tracking guidance, not permission to override repository, user, or orchestrator instructions.

- **Conservative (default)**: Use `bd` for task tracking. Do not run git commits, git pushes, or Dolt remote sync unless explicitly asked. At handoff, report changed files, validation, and suggested next commands.
- **Minimal**: Keep tool instruction files as pointers to `bd prime`; use the same conservative git policy unless active instructions say otherwise.
- **Team-maintainer**: Only when the repository explicitly opts in, agents may close beads, run quality gates, commit, and push as part of session close. A current "do not commit" or "do not push" instruction still wins.

## Session Completion

This protocol applies when ending a Beads implementation workflow. It is subordinate to explicit user, repository, and orchestrator instructions.

1. **File issues for remaining work** - Create beads for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **Handle git/sync by active profile**:
   ```bash
   # Conservative/minimal/default: report status and proposed commands; wait for approval.
   git status

   # Team-maintainer opt-in only, unless current instructions forbid it:
   git pull --rebase
   git push
   git status
   ```

5. **Hand off** - Summarize changes, validation, issue status, and any blocked sync/commit/push step

**Critical rules:**

- Explicit user or orchestrator instructions override this Beads block.
- Do not commit or push without clear authority from the active profile or the current user request.
- If a required sync or push is blocked, stop and report the exact command and error.

<!-- END BEADS INTEGRATION -->

## Build & Test

_Add your build and test commands here_

```bash

# Example:

# npm install

# npm test

```

## Architecture Overview

_Add a brief overview of your project architecture_

## Conventions & Patterns

_Add your project-specific conventions here_

<!-- GSD:project-start source:PROJECT.md -->

## Project

**Platform Maturity Audit Agent — Production Ready**

A Go platform maturity audit agent that measures infrastructure standardization, CI/CD maturity, test coverage, and cloud FinOps waste across an engineering org’s repositories — then ranks actionable findings for leadership and FinOps. Today it ships as a working local/demo binary (`POST /audit/trigger`, supervisor → parallel workers → aggregator → output). This milestone takes it to **full TDD parity**: live integrations, production ops, and leadership-grade narrative outputs.

**Core Value:** Engineering leadership can trust a scheduled audit run to produce accurate, persisted, ranked findings from live GitHub/GitLab + AWS data — without manual audit effort.

### Constraints

- **Tech stack**: Stay on Go + stdlib HTTP orchestrator; add drivers/SDKs only as needed (pgx, AWS SDK, Slack, GitHub API client, LLM SDK)
- ** determinism**: Metric collection remains deterministic code; LLM only for executive summary / narrative
- **Security**: Read-only GitHub/GitLab tokens; AWS ReadOnly + Cost Explorer; no secrets in repo
- **Performance**: Full audit target &lt;15 min for ~28 repos; FinOps-only &lt;3 min; incremental &lt;90s (from TDD)
- **Compatibility**: Keep demo sources and in-memory path for tests/local demos alongside live mode
- **Docs**: TDD.md remains design reference; README must describe what the binary actually does
- **Task tracking**: Beads (`bd`) is the durable source of truth for progress, blockers, and handoffs — not markdown TODOs
- **Quality bar**: golang-pro + test-master — gofmt/golangci-lint, context on blocking ops, table-driven tests with `-race`, ≥80% coverage on changed packages; code-documenter — GoDoc on all exported APIs

<!-- GSD:project-end -->

<!-- GSD:stack-start source:codebase/STACK.md -->

## Technology Stack

## Languages

- Go 1.26.2 - All application code under `cmd/` and `internal/`
- SQL - Schema definition in `migrations/001_schema.sql` (PostgreSQL dialect; not executed by the Go binary today)
- Bash - Smoke tests in `scripts/test-audit-trigger.sh`
- JSON - Repo audit manifest in `configs/repos.json`
- Markdown - Design/reference docs (`TDD.md`, `README.md`)

## Runtime

- Go toolchain 1.26.2 (module directive in `go.mod`)
- Standard library `net/http` server — no separate app runtime beyond the compiled binary
- Docker multi-stage build uses `golang:1.26-alpine` (builder) and `alpine:3.20` (runtime) in `Dockerfile`
- Go modules (`go mod`)
- Lockfile: `go.sum` present

## Frameworks

- None — vanilla Go HTTP server (`net/http.ServeMux` in `internal/api/handler.go`)
- Orchestration is a custom Go “LangGraph-equivalent” agent (`internal/agent/agent.go`), not the Python LangGraph library referenced in `TDD.md` / `README.md`
- Go standard `testing` package — unit, package, and e2e-style tests co-located as `*_test.go`
- Race detector via `go test -race` (`Makefile` target `test`)
- Coverage profiles via `-coverprofile=coverage.out`
- `go build` — produces `bin/agent` from `cmd/agent` (`Makefile` targets `build`, `run`)
- `gofmt` — formatting (`Makefile` target `fmt`)
- `go vet` — static checks (`Makefile` target `vet`)
- `golangci-lint` — lint entrypoint (`Makefile` target `lint`); no `.golangci.yml` committed
- Docker — container image build (`Dockerfile`)

## Key Dependencies

- `github.com/google/uuid` v1.6.0 - Audit run IDs in `internal/supervisor/supervisor.go`
- `gopkg.in/yaml.v3` v3.0.1 - CI/CD pipeline YAML parsing in `internal/cicd/auditor.go`
- Go stdlib `net/http` - HTTP listen/serve and `/audit/trigger` routing (`cmd/agent/main.go`, `internal/api/handler.go`)
- Go stdlib `encoding/json` - Manifest load and API request/response bodies
- Go stdlib `sync` - Parallel worker fan-out in `internal/agent/agent.go`
- No database driver, AWS SDK, Slack SDK, or GitHub/GitLab client modules in `go.mod`

## Configuration

- No required secrets or `.env` files for the current binary
- CLI flags only (`cmd/agent/main.go`):
- Smoke script optional overrides (`scripts/test-audit-trigger.sh`): `AUDIT_ADDR`, `AUDIT_MANIFEST`, `AUDIT_BIN`, `AUDIT_READY_TIMEOUT`
- `go.mod` / `go.sum` — module identity and dependency pins
- `Dockerfile` — multi-stage static binary (`CGO_ENABLED=0`)
- `Makefile` — test, lint, vet, fmt, build, run, smoke
- `configs/repos.json` — audited repository manifest (name, owner, provider, default_branch)

## Platform Requirements

- Go 1.26.2+ toolchain
- Any OS supported by Go (developed/tested on macOS/Linux)
- Optional: `golangci-lint` for `make lint`, Docker for image builds, `curl` + `python3` for `make smoke`
- Static Linux binary in Alpine container (`Dockerfile`: exposes port 8080, entrypoint `/app/agent`)
- Designed deployment target (per `TDD.md`): Kubernetes CronJob + long-running HTTP for triggers — not wired as manifests in this repo
- Current process wiring uses in-memory store + log-based Slack stub (`cmd/agent/main.go`); no live Postgres/AWS/Slack clients at runtime

<!-- GSD:stack-end -->

<!-- GSD:conventions-start source:CONVENTIONS.md -->

## Conventions

## Naming Patterns

- Lowercase single-word package directories under `internal/` (`agent`, `cicd`, `finops`, `reposcanner`)
- Implementation: descriptive noun (`handler.go`, `scanner.go`, `auditor.go`, `analyzer.go`, `writer.go`, `manifest.go`, `types.go`)
- Tests: `*_test.go` co-located with the package under test (`handler_test.go`, `scanner_test.go`)
- Entry point: `cmd/agent/main.go`
- Exported constructors: `New`, `NewHandler` (`internal/agent/agent.go`, `internal/api/handler.go`)
- Exported operations: verb phrases — `Run`, `Plan`, `Scan`, `Analyze`, `ScorePipeline`, `Aggregate`, `Write`, `LoadManifest`
- Unexported helpers: camelCase verbs — `toSet`, `dedupe`, `clamp`, `buildDigest`, `gapFinding`, `cloudFinding`
- HTTP method handlers: short lowercase — `trigger` on `*Handler`
- No `Get`/`Set` prefixes; field access is direct on structs
- camelCase locals (`auditRunID`, `fileCalls`, `wantCache`)
- Short idiomatic names in narrow scope (`err`, `ctx`, `w`, `r`, `tt`, `mu`, `wg`)
- Unexported package-level vars for tables/config (`dimensionPaths`, `secretPatterns`, `lineThreshold`)
- PascalCase structs and interfaces, no `I` prefix (`Runner`, `Dependencies`, `FileChecker`, `Store`, `Notifier`)
- Domain enums as typed string constants: `Scope`, `Severity`, `FindingType`, `FindingStatus` in `internal/domain/types.go`
- Const values: `ScopeFull`, `SeverityCritical`, `FindingTypeCICDGap` (type prefix + descriptive suffix)
- Result DTOs named `Result`, `Output`, `PlanResult`, `TriggerRequest` / `TriggerResponse`

## Code Style

- Standard Go formatting via `gofmt` (`make fmt` → `gofmt -w .`)
- Tabs for indentation (gofmt default)
- Double quotes for strings; raw string literals (`` ` ``) for multi-line YAML fixtures in tests
- No project `.editorconfig` or custom gofmt options detected
- `go vet ./...` via `make vet`
- `golangci-lint run ./...` via `make lint` (no checked-in `.golangci.yml` — use golangci-lint defaults)
- Run lint/vet before considering a change complete

## Import Organization

- Blank line between stdlib, third-party, and internal groups
- Prefer `goimports` / editor organize-imports to keep groups sorted
- No import aliases unless required for collision
- Not applicable — use full module path `github.com/tazzledazzle/build-agent-smith/internal/<pkg>`

## Error Handling

- Return `(T, error)` from exported operations; never panic in library code
- Wrap with context via `fmt.Errorf("operation: %w", err)` — see `internal/config/manifest.go`, `internal/reposcanner/scanner.go`, `internal/output/writer.go`
- Check `ctx.Err()` at the start of long-running node functions (`Plan`, `Scan`, `Analyze`, `Write`)
- HTTP boundary: map errors to status codes with `http.Error` in `internal/api/handler.go` (400 validation, 405 method, 500 runner failure)
- Non-fatal worker failures: append `domain.AuditError` and set `Status` to `"PARTIAL_AUDIT"` rather than failing the whole run (`internal/agent/agent.go`)
- Prefer plain `error` with descriptive messages; no custom error types detected
- Validation failures return early with clear messages (`plan: incremental scope requires target repo`)
- Ignore intentional encode errors only when documented (`_ = json.NewEncoder(w).Encode(resp)` after headers set)

## Logging

- Standard library `log` package in `cmd/agent/main.go` only
- Levels: `log.Printf` for info, `log.Fatalf` for fatal startup/server errors
- Log at process boundaries (listen address, shutdown, Slack digest stub)
- Library packages (`internal/*`) do not log — return errors to callers
- Demo Slack notifier prints digests via `log.Printf("slack digest:\n%s", text)`

## Comments

- Package doc comment on every package: `// Package X ...` as the first line of the primary `.go` file
- Exported types and functions get a one-line doc comment starting with the name (`// New creates an Agent...`)
- Inline comments explain non-obvious rubric math or scoring edge cases (e.g. zero-effort floor in `PriorityScore`)
- Avoid restating the obvious next to simple assignments
- Required for all exported identifiers
- Prefer complete sentences; link related types by name in prose
- None present in the codebase; add `// TODO: description` only when tracking unfinished work, and prefer issues for lasting debt

## Function Design

- Keep exported entry points focused; extract unexported helpers for scoring dimensions (`scoreCaching`, `scoreTestGate`, etc. in `internal/cicd/auditor.go`)
- Prefer early returns for validation and context cancellation
- First parameter is `context.Context` for any operation that may block, cancel, or call I/O
- Pass domain structs (`domain.RepoConfig`, `domain.AuditState`) rather than long primitive lists
- Construct request structs for multi-field inputs (`agent.RunRequest`, `supervisor.Request`, `aggregator.Input`)
- Prefer pointers for mutable/aggregated results (`*Result`, `*domain.AuditState`)
- Return `nil, err` on failure; never return a partial success pointer with a non-nil error unless documented
- Use named result fields on structs instead of multiple return values beyond `(T, error)`

## Module Design

- Narrow public surface per package: constructors + one primary operation
- Define small consumer-owned interfaces at the call site (`api.Runner`, `output.Store`, `output.Notifier`, `reposcanner.FileChecker`, `agent.Dependencies`)
- Accept interfaces, return concrete types (`*Agent`, `*Handler`, `*Writer`)
- Not used — import the specific `internal/<pkg>` package
- Shared types live only in `internal/domain`; workers must not import each other sideways when domain types suffice
- `cmd/agent` wires dependencies; keep orchestration out of worker packages

<!-- GSD:conventions-end -->

<!-- GSD:architecture-start source:ARCHITECTURE.md -->

## Architecture

## System Overview

```text

```

## Component Responsibilities

| Component | Responsibility | File |
|-----------|----------------|------|
| Main / wiring | Flags, manifest load, DI, HTTP server lifecycle | `cmd/agent/main.go` |
| API handler | Validate trigger body; invoke `Runner` | `internal/api/handler.go` |
| Supervisor | Scope → worker set + repo partition; mint `audit_run_id` | `internal/supervisor/supervisor.go` |
| Agent | Fan-out workers in parallel; merge metrics/findings | `internal/agent/agent.go` |
| Repo scanner | 0–5 standardization rubric via file presence | `internal/reposcanner/scanner.go` |
| CI/CD auditor | Parse pipeline YAML; 6-dimension maturity score | `internal/cicd/auditor.go` |
| Coverage analyzer | Threshold + change-risk findings | `internal/coverage/analyzer.go` |
| FinOps analyzer | Idle / over-provisioned / orphaned / untagged flags | `internal/finops/analyzer.go` |
| Aggregator | Dedupe, priority rank, DORA + repo scores | `internal/aggregator/aggregator.go` |
| Output writer | Persist run/scores/findings; post digest | `internal/output/writer.go` |
| Memory store | In-process `output.Store` for local/demo | `internal/store/memory.go` |
| Demo sources | Fixture implementation of `agent.Dependencies` | `internal/demo/sources.go` |
| Domain | Shared enums, `Finding`, `AuditState`, priority math | `internal/domain/types.go` |
| Config | Load JSON repo manifest | `internal/config/manifest.go` |

## Pattern Overview

- One executable (`cmd/agent`) serves `POST /audit/trigger` and runs audits synchronously in-request
- Scope-driven planning (`full` | `incremental` | `finops_only`) selects workers and repos
- Worker packages are pure analysis functions/structs; I/O is behind interfaces
- Shared immutable-ish state object `domain.AuditState` is built after workers complete
- Persistence is interface-based (`output.Store`); runtime wires `store.Memory` today
- Target schema for Postgres lives in `migrations/001_schema.sql` (not wired in `main`)

## Layers

- Purpose: Accept audit triggers and return run summaries
- Location: `internal/api/`
- Contains: `Handler`, `TriggerRequest`/`TriggerResponse`, `Runner` interface
- Depends on: `internal/domain` only (no worker imports)
- Used by: `cmd/agent/main.go` HTTP server
- Purpose: Bootstrap config, dependencies, and adapters
- Location: `cmd/agent/main.go`
- Contains: Flag parsing, `agentRunner` (Agent + Writer), `logNotifier`, graceful shutdown
- Depends on: `agent`, `api`, `config`, `demo`, `output`, `store`
- Used by: Process entry / Docker `ENTRYPOINT`
- Purpose: Plan scope and execute workers in parallel
- Location: `internal/supervisor/`, `internal/agent/`
- Contains: `supervisor.Plan`, `Agent.Run`, `Dependencies` interface
- Depends on: Workers, aggregator, domain
- Used by: `agentRunner` in main (via `api.Runner`)
- Purpose: Produce findings and per-repo metrics from inputs
- Location: `internal/reposcanner/`, `internal/cicd/`, `internal/coverage/`, `internal/finops/`
- Contains: Rubric scorers and analyzers returning `[]domain.Finding`
- Depends on: `internal/domain` (+ YAML for cicd); reposcanner also needs `FileChecker`
- Used by: `internal/agent`
- Purpose: Merge worker outputs into ranked findings and scores
- Location: `internal/aggregator/`
- Contains: Dedup by `(repo, type, title, resource_id)`, priority sort, DORA composite
- Depends on: `internal/domain`
- Used by: `internal/agent` after `WaitGroup`
- Purpose: Persist results and notify
- Location: `internal/output/`, `internal/store/`
- Contains: `Writer`, `Store`/`Notifier` interfaces, `Memory` adapter
- Depends on: `internal/domain`
- Used by: `agentRunner` after successful `Agent.Run`
- Purpose: Shared types and ranking helpers with no outbound internal imports
- Location: `internal/domain/`
- Contains: `Scope`, `Finding`, `AuditState`, `PriorityScore`, severity weights
- Depends on: stdlib only
- Used by: All other internal packages
- Purpose: Manifest loading and deterministic external data for demos/tests
- Location: `internal/config/`, `internal/demo/`
- Contains: JSON manifest loader; fixture `Dependencies`
- Depends on: domain (+ agent/finops for demo)
- Used by: main, e2e tests

## Data Flow

### Primary Request Path (HTTP audit trigger)

### Scope Routing

| Scope | Workers | Repos |
|-------|---------|-------|
| `full` | repo_scanner, cicd_auditor, coverage_analyzer, cloud_finops | All manifest repos |
| `incremental` | repo_scanner, cicd_auditor, coverage_analyzer | Single `TargetRepoName` |
| `finops_only` | cloud_finops | Manifest repos (FinOps is org-level inventory) |

### Secondary Flow (Programmatic / tests)

- Per-run state is a local `domain.AuditState` pointer built during `Agent.Run`
- Concurrent worker writes guarded by `sync.Mutex` in the agent
- Persistence is append-only into `store.Memory` (process-lifetime); Postgres schema is defined but unused at runtime
- No shared global agent state across requests beyond the in-memory store singleton wired in `main`

## Key Abstractions

- Purpose: Shared run result (LangGraph-state analogue)
- Examples: `internal/domain/types.go` (`AuditState`, `Finding`, `RepoScore`, `DoraScore`)
- Pattern: Plain struct passed by pointer after orchestration completes
- Purpose: Abstract GitHub/Codecov/AWS (and file presence) fetches
- Examples: `agent.Dependencies` in `internal/agent/agent.go`; implemented by `demo.Sources`
- Pattern: Interface injection at `agent.New`
- Purpose: Deterministic scope → workers/repos routing
- Examples: `supervisor.Plan` / `PlanResult` in `internal/supervisor/supervisor.go`
- Pattern: Pure function (no LLM); UUID run ID generation
- Purpose: Uniform findings + metric contribution per analysis node
- Examples: `reposcanner.Result`, `cicd.Result`, `coverage.Result`, `finops.Result`
- Pattern: Package-local Result structs + `[]domain.Finding`
- Purpose: Keep API and output layers testable without full graph
- Examples: `api.Runner`, `output.Store`, `output.Notifier`
- Pattern: Small interfaces defined next to consumers; adapters in main/store/demo
- Purpose: Rank findings by `cost × severity_weight / effort`
- Examples: `domain.PriorityScore` / `SeverityWeight` in `internal/domain/types.go`
- Pattern: Shared pure functions used by workers and aggregator

## Entry Points

- Location: `cmd/agent/main.go`
- Triggers: `go run` / `make run` / Docker; listens default `:8080`
- Responsibilities: Load `configs/repos.json`, wire demo deps + memory store + handler, serve until SIGINT/SIGTERM
- Location: `internal/api/handler.go` → `POST /audit/trigger`
- Triggers: Scheduler/webhook/manual HTTP clients (`make smoke` → `scripts/test-audit-trigger.sh`)
- Responsibilities: Validate JSON body, run audit synchronously, return summary JSON
- Location: `internal/agent.Agent.Run`
- Triggers: Unit/e2e tests and any future CLI wrapping the same graph
- Responsibilities: Full supervisor → workers → aggregator cycle without HTTP

## Architectural Constraints

- **Threading:** Workers run in goroutines with `sync.WaitGroup`; shared findings/metrics/errors protected by one mutex. Do not share mutable maps without locking.
- **Global state:** No package-level mutable singletons in libraries; only process-scoped `store.Memory` and manifest slice held by `api.Handler` / main.
- **Circular imports:** Workers and aggregator must not import `internal/agent`. `demo` may import `agent` (implements `Dependencies`). Domain must stay leaf-level.
- **Synchronous HTTP:** Audit completes inside the request; do not assume background job queue exists.
- **I/O at edges:** Analysis packages accept already-fetched YAML/coverage/inventory; only agent/reposcanner call dependency interfaces for fetch.

## Anti-Patterns

### Putting business logic in `cmd/agent` or `api`

### Calling workers without `supervisor.Plan`

### Importing `agent` from workers

### Ignoring worker errors

## Error Handling

- API: `400` for bad JSON/invalid scope/missing incremental repo; `405` wrong method; `500` with error text for run failures (`internal/api/handler.go`)
- Supervisor: Return `fmt.Errorf` for empty manifest, unknown scope, missing incremental target
- Workers: Return errors upward; agent records `AuditError{Node, Message}` and continues other goroutines
- Context: Check `ctx.Err()` at plan/aggregate/write boundaries and inside long-running loops
- Output: Wrap store/notifier failures with stage prefixes (`save audit run`, `slack digest`)

## Cross-Cutting Concerns

- Standard library `log` in `cmd/agent/main.go` (listen address, shutdown, Slack digest text via `logNotifier`)
- No structured logger package yet; workers do not log
- Manifest: non-empty `repos` at load (`internal/config/manifest.go`)
- Trigger body: scope enum + incremental requires `repo` (`internal/api/handler.go`)
- Supervisor: empty repos / unknown scope / incremental target membership
- Not implemented on `/audit/trigger` (open local/demo endpoint)
- External API tokens are design-only (TDD); demo sources need no secrets
- Logical schema: `migrations/001_schema.sql` (`audit_runs`, `repo_scores`, `findings`, `platform_health_current`)
- Runtime adapter: `store.Memory` implementing `output.Store`

<!-- GSD:architecture-end -->

<!-- GSD:skills-start source:skills/ -->

## Project Skills

| Skill | Description | Path |
|-------|-------------|------|
| beads | Use when working in a repository that uses bd or Beads for durable project task tracking, issue dependencies, blocker management, multi-session handoff, or shared work memory. Trigger when the user asks to find ready work, claim or close tasks, create follow-up work, inspect blockers, recover project context, or choose between local planning and persistent project tracking. | `.agents/skills/beads/SKILL.md` |
<!-- GSD:skills-end -->

<!-- GSD:workflow-start source:GSD defaults -->

## GSD Workflow Enforcement

Before using Edit, Write, or other file-changing tools, start work through a GSD command so planning artifacts and execution context stay in sync.

Use these entry points:

- `/gsd-quick` for small fixes, doc updates, and ad-hoc tasks
- `/gsd-debug` for investigation and bug fixing
- `/gsd-execute-phase` for planned phase work

Do not make direct repo edits outside a GSD workflow unless the user explicitly asks to bypass it.
<!-- GSD:workflow-end -->

<!-- GSD:profile-start -->

## Developer Profile

> Profile not yet configured. Run `/gsd-profile-user` to generate your developer profile.
> This section is managed by `generate-claude-profile` -- do not edit manually.
<!-- GSD:profile-end -->
