<!-- refreshed: 2026-07-12 -->
# Architecture

**Analysis Date:** 2026-07-12

## System Overview

```text
┌─────────────────────────────────────────────────────────────┐
│                    HTTP / Process Entry                      │
│              `cmd/agent/main.go`  →  `internal/api`          │
│                     POST /audit/trigger                      │
└────────────────────────────┬────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────┐
│                   Orchestration Layer                        │
│  Supervisor plan (`internal/supervisor`)                     │
│  Parallel agent run (`internal/agent`)                       │
├──────────────┬──────────────┬──────────────┬────────────────┤
│ Repo Scanner │ CI/CD Auditor│ Coverage     │ Cloud FinOps   │
│ `reposcanner`│ `cicd`       │ `coverage`   │ `finops`       │
└──────┬───────┴──────┬───────┴──────┬───────┴────────┬───────┘
       │              │              │                │
       └──────────────┴──────┬───────┴────────────────┘
                             ▼
┌─────────────────────────────────────────────────────────────┐
│              Aggregator (`internal/aggregator`)              │
│         Deduplicate · Priority rank · DORA scores            │
└────────────────────────────┬────────────────────────────────┘
                             ▼
┌─────────────────────────────────────────────────────────────┐
│  Output Writer (`internal/output`) → Store / Notifier        │
│  `internal/store` (Memory) · Slack digest (log notifier)     │
└─────────────────────────────────────────────────────────────┘
```

Shared types live in `internal/domain`. External data is injected via `agent.Dependencies` (demo fixtures in `internal/demo`, or future GitHub/Codecov/AWS clients).

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

**Overall:** Single-binary HTTP service with LangGraph-inspired supervisor → parallel workers → aggregator → output topology (implemented in Go, not Python LangGraph).

**Key Characteristics:**
- One executable (`cmd/agent`) serves `POST /audit/trigger` and runs audits synchronously in-request
- Scope-driven planning (`full` | `incremental` | `finops_only`) selects workers and repos
- Worker packages are pure analysis functions/structs; I/O is behind interfaces
- Shared immutable-ish state object `domain.AuditState` is built after workers complete
- Persistence is interface-based (`output.Store`); runtime wires `store.Memory` today
- Target schema for Postgres lives in `migrations/001_schema.sql` (not wired in `main`)

## Layers

**Transport / API Layer:**
- Purpose: Accept audit triggers and return run summaries
- Location: `internal/api/`
- Contains: `Handler`, `TriggerRequest`/`TriggerResponse`, `Runner` interface
- Depends on: `internal/domain` only (no worker imports)
- Used by: `cmd/agent/main.go` HTTP server

**Composition / Wiring Layer:**
- Purpose: Bootstrap config, dependencies, and adapters
- Location: `cmd/agent/main.go`
- Contains: Flag parsing, `agentRunner` (Agent + Writer), `logNotifier`, graceful shutdown
- Depends on: `agent`, `api`, `config`, `demo`, `output`, `store`
- Used by: Process entry / Docker `ENTRYPOINT`

**Orchestration Layer:**
- Purpose: Plan scope and execute workers in parallel
- Location: `internal/supervisor/`, `internal/agent/`
- Contains: `supervisor.Plan`, `Agent.Run`, `Dependencies` interface
- Depends on: Workers, aggregator, domain
- Used by: `agentRunner` in main (via `api.Runner`)

**Worker / Analysis Layer:**
- Purpose: Produce findings and per-repo metrics from inputs
- Location: `internal/reposcanner/`, `internal/cicd/`, `internal/coverage/`, `internal/finops/`
- Contains: Rubric scorers and analyzers returning `[]domain.Finding`
- Depends on: `internal/domain` (+ YAML for cicd); reposcanner also needs `FileChecker`
- Used by: `internal/agent`

**Aggregation Layer:**
- Purpose: Merge worker outputs into ranked findings and scores
- Location: `internal/aggregator/`
- Contains: Dedup by `(repo, type, title, resource_id)`, priority sort, DORA composite
- Depends on: `internal/domain`
- Used by: `internal/agent` after `WaitGroup`

**Output Layer:**
- Purpose: Persist results and notify
- Location: `internal/output/`, `internal/store/`
- Contains: `Writer`, `Store`/`Notifier` interfaces, `Memory` adapter
- Depends on: `internal/domain`
- Used by: `agentRunner` after successful `Agent.Run`

**Domain Layer:**
- Purpose: Shared types and ranking helpers with no outbound internal imports
- Location: `internal/domain/`
- Contains: `Scope`, `Finding`, `AuditState`, `PriorityScore`, severity weights
- Depends on: stdlib only
- Used by: All other internal packages

**Config / Demo Adapters:**
- Purpose: Manifest loading and deterministic external data for demos/tests
- Location: `internal/config/`, `internal/demo/`
- Contains: JSON manifest loader; fixture `Dependencies`
- Depends on: domain (+ agent/finops for demo)
- Used by: main, e2e tests

## Data Flow

### Primary Request Path (HTTP audit trigger)

1. Client `POST /audit/trigger` with optional `{"scope","repo"}` (`internal/api/handler.go`)
2. Handler defaults scope to `full`, validates scope/repo, calls `Runner.Run`
3. `agentRunner.Run` in `cmd/agent/main.go` calls `agent.Agent.Run`
4. `supervisor.Plan` selects workers + repos and generates UUID `AuditRunID` (`internal/supervisor/supervisor.go`)
5. Agent spawns goroutines per selected worker×repo (and one FinOps job if planned) (`internal/agent/agent.go`)
6. Workers fetch via `Dependencies` / analyze / append findings + metrics under a mutex
7. `aggregator.Aggregate` dedupes, ranks by `domain.PriorityScore`, builds DORA/repo scores
8. Agent sets `Status` to `COMPLETE` or `PARTIAL_AUDIT` if any worker errors
9. `output.Writer.Write` saves audit run, repo scores, findings; posts Slack digest via `Notifier`
10. Handler returns JSON `{audit_run_id, status, finding_count, scope}`

### Scope Routing

| Scope | Workers | Repos |
|-------|---------|-------|
| `full` | repo_scanner, cicd_auditor, coverage_analyzer, cloud_finops | All manifest repos |
| `incremental` | repo_scanner, cicd_auditor, coverage_analyzer | Single `TargetRepoName` |
| `finops_only` | cloud_finops | Manifest repos (FinOps is org-level inventory) |

### Secondary Flow (Programmatic / tests)

1. Construct `agent.New(deps)` with stub or `demo.Sources`
2. Call `Agent.Run` with `RunRequest`
3. Optionally `output.New(store, notifier).Write(ctx, state)`
4. Assert on `AuditState` / store contents (`internal/agent/e2e_test.go`, `internal/api/live_trigger_test.go`)

**State Management:**
- Per-run state is a local `domain.AuditState` pointer built during `Agent.Run`
- Concurrent worker writes guarded by `sync.Mutex` in the agent
- Persistence is append-only into `store.Memory` (process-lifetime); Postgres schema is defined but unused at runtime
- No shared global agent state across requests beyond the in-memory store singleton wired in `main`

## Key Abstractions

**AuditState:**
- Purpose: Shared run result (LangGraph-state analogue)
- Examples: `internal/domain/types.go` (`AuditState`, `Finding`, `RepoScore`, `DoraScore`)
- Pattern: Plain struct passed by pointer after orchestration completes

**Dependencies (ports):**
- Purpose: Abstract GitHub/Codecov/AWS (and file presence) fetches
- Examples: `agent.Dependencies` in `internal/agent/agent.go`; implemented by `demo.Sources`
- Pattern: Interface injection at `agent.New`

**Supervisor Plan:**
- Purpose: Deterministic scope → workers/repos routing
- Examples: `supervisor.Plan` / `PlanResult` in `internal/supervisor/supervisor.go`
- Pattern: Pure function (no LLM); UUID run ID generation

**Worker Result:**
- Purpose: Uniform findings + metric contribution per analysis node
- Examples: `reposcanner.Result`, `cicd.Result`, `coverage.Result`, `finops.Result`
- Pattern: Package-local Result structs + `[]domain.Finding`

**Runner / Store / Notifier:**
- Purpose: Keep API and output layers testable without full graph
- Examples: `api.Runner`, `output.Store`, `output.Notifier`
- Pattern: Small interfaces defined next to consumers; adapters in main/store/demo

**PriorityScore:**
- Purpose: Rank findings by `cost × severity_weight / effort`
- Examples: `domain.PriorityScore` / `SeverityWeight` in `internal/domain/types.go`
- Pattern: Shared pure functions used by workers and aggregator

## Entry Points

**HTTP server binary:**
- Location: `cmd/agent/main.go`
- Triggers: `go run` / `make run` / Docker; listens default `:8080`
- Responsibilities: Load `configs/repos.json`, wire demo deps + memory store + handler, serve until SIGINT/SIGTERM

**Audit trigger endpoint:**
- Location: `internal/api/handler.go` → `POST /audit/trigger`
- Triggers: Scheduler/webhook/manual HTTP clients (`make smoke` → `scripts/test-audit-trigger.sh`)
- Responsibilities: Validate JSON body, run audit synchronously, return summary JSON

**Package-level library use:**
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

**What happens:** Scoring or YAML parsing appears in main or HTTP handlers.
**Why it's wrong:** Breaks layering; handlers become untestable without HTTP; workers cannot be reused.
**Do this instead:** Keep handlers thin (`api.Runner`); put analysis in worker packages; wire only in `cmd/agent/main.go`.

### Calling workers without `supervisor.Plan`

**What happens:** Hard-coding worker lists in the agent or API.
**Why it's wrong:** Scope rules (`incremental` requires target; `finops_only` skips repo workers) drift and duplicate.
**Do this instead:** Always route through `supervisor.Plan` (`internal/supervisor/supervisor.go`).

### Importing `agent` from workers

**What happens:** `cicd`/`coverage`/`finops` import orchestration types.
**Why it's wrong:** Creates cycles and couples pure analysis to the graph.
**Do this instead:** Workers depend only on `domain` (and stdlib/yaml). Adapters live in `demo` or future `internal/clients`.

### Ignoring worker errors

**What happens:** Returning only fatal aggregator errors and dropping per-worker failures.
**Why it's wrong:** Partial audits look complete; FinOps/CI gaps go silent.
**Do this instead:** Append `domain.AuditError` and set `Status` to `PARTIAL_AUDIT` when any worker fails (`internal/agent/agent.go`).

## Error Handling

**Strategy:** Fail fast on plan/config/validation; collect non-fatal worker errors; fail request on orchestration/output errors.

**Patterns:**
- API: `400` for bad JSON/invalid scope/missing incremental repo; `405` wrong method; `500` with error text for run failures (`internal/api/handler.go`)
- Supervisor: Return `fmt.Errorf` for empty manifest, unknown scope, missing incremental target
- Workers: Return errors upward; agent records `AuditError{Node, Message}` and continues other goroutines
- Context: Check `ctx.Err()` at plan/aggregate/write boundaries and inside long-running loops
- Output: Wrap store/notifier failures with stage prefixes (`save audit run`, `slack digest`)

## Cross-Cutting Concerns

**Logging:**
- Standard library `log` in `cmd/agent/main.go` (listen address, shutdown, Slack digest text via `logNotifier`)
- No structured logger package yet; workers do not log

**Validation:**
- Manifest: non-empty `repos` at load (`internal/config/manifest.go`)
- Trigger body: scope enum + incremental requires `repo` (`internal/api/handler.go`)
- Supervisor: empty repos / unknown scope / incremental target membership

**Authentication:**
- Not implemented on `/audit/trigger` (open local/demo endpoint)
- External API tokens are design-only (TDD); demo sources need no secrets

**Persistence contract:**
- Logical schema: `migrations/001_schema.sql` (`audit_runs`, `repo_scores`, `findings`, `platform_health_current`)
- Runtime adapter: `store.Memory` implementing `output.Store`

---

*Architecture analysis: 2026-07-12*
*Update when major patterns change*
