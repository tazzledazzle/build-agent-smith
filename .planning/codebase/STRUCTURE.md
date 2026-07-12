# Codebase Structure

**Analysis Date:** 2026-07-12

## Directory Layout

```
build-agent-smith/
├── cmd/
│   └── agent/              # HTTP server main package
├── internal/
│   ├── agent/              # Parallel orchestrator (LangGraph-equivalent)
│   ├── aggregator/         # Dedupe, rank, DORA scoring
│   ├── api/                # POST /audit/trigger HTTP handler
│   ├── cicd/               # CI/CD pipeline maturity auditor
│   ├── config/             # Repo manifest JSON loader
│   ├── coverage/           # Coverage + change-risk analyzer
│   ├── demo/               # Fixture Dependencies for local demos
│   ├── domain/             # Shared types and priority math
│   ├── finops/             # Cloud waste analyzer
│   ├── output/             # Persist + Slack digest writer
│   ├── reposcanner/        # Standardization file-presence scanner
│   ├── store/              # In-memory Store adapter
│   └── supervisor/         # Scope → workers/repos planner
├── configs/                # Checked-in repo manifest
├── migrations/             # Postgres DDL (reference schema)
├── scripts/                # Smoke / integration shell scripts
├── testdata/               # Reserved for fixtures (empty)
├── bin/                    # Build output (`agent` binary; gitignored)
├── Dockerfile              # Multi-stage Go image
├── Makefile                # test, build, run, smoke targets
├── go.mod / go.sum         # Module github.com/tazzledazzle/build-agent-smith
├── README.md               # Architecture mermaid + portfolio blurb
└── TDD.md                  # Technical design document
```

## Directory Purposes

**cmd/agent/**
- Purpose: Process entry point for the audit HTTP server
- Contains: `main.go` only
- Key files: `cmd/agent/main.go` — flags (`-addr`, `-manifest`), DI wiring, graceful shutdown
- Subdirectories: None

**internal/**
- Purpose: All non-exportable application packages (standard Go layout)
- Contains: One package per concern; co-located `*_test.go`
- Key files: See package list below
- Subdirectories: Flat feature packages (no nested feature trees)

**internal/agent/**
- Purpose: Orchestrate supervisor plan + parallel workers + aggregation
- Contains: `agent.go`, `agent_test.go`, `e2e_test.go`
- Key files: `agent.go` — `Dependencies`, `Agent.Run`

**internal/supervisor/**
- Purpose: Plan audit scope and worker routing
- Contains: `supervisor.go`, `supervisor_test.go`
- Key files: `supervisor.go` — `Plan`, `PlanResult`

**internal/api/**
- Purpose: REST transport for audit triggers
- Contains: `handler.go`, `handler_test.go`, `live_trigger_test.go`
- Key files: `handler.go` — `POST /audit/trigger`

**internal/reposcanner/, cicd/, coverage/, finops/**
- Purpose: Worker/analysis nodes
- Contains: Single primary `.go` implementation + `_test.go` each
- Key files:
  - `reposcanner/scanner.go`
  - `cicd/auditor.go`
  - `coverage/analyzer.go`
  - `finops/analyzer.go`

**internal/aggregator/**
- Purpose: Merge and rank findings; compute DORA/repo scores
- Contains: `aggregator.go`, `aggregator_test.go`

**internal/output/, store/**
- Purpose: Output-writer node and persistence adapters
- Contains: Writer + interfaces; Memory store
- Key files: `output/writer.go`, `store/memory.go`

**internal/domain/**
- Purpose: Shared domain model (leaf package)
- Contains: `types.go`, `domain_test.go`

**internal/config/**
- Purpose: Load repository audit manifest
- Contains: `manifest.go`, `manifest_test.go`, `repos_manifest_test.go`

**internal/demo/**
- Purpose: Deterministic fake external sources
- Contains: `sources.go` (no separate test file; covered via agent e2e)

**configs/**
- Purpose: Runtime repository list for audits
- Contains: JSON manifests
- Key files: `configs/repos.json` (5 demo repos)

**migrations/**
- Purpose: Target Postgres schema matching TDD persistence model
- Contains: SQL migration files
- Key files: `migrations/001_schema.sql`

**scripts/**
- Purpose: Manual smoke checks against a running server
- Contains: Shell scripts
- Key files: `scripts/test-audit-trigger.sh` (`make smoke`)

**testdata/**
- Purpose: Reserved for shared fixtures
- Contains: Currently empty
- Key files: None

**bin/**
- Purpose: Compiled binary output from `make build`
- Contains: `agent` executable
- Generated: Yes — listed in `.gitignore`

**.planning/**
- Purpose: GSD planning artifacts (codebase maps, phases)
- Contains: `codebase/` analysis docs
- Committed: Typically yes when using GSD workflows

## Key File Locations

**Entry Points:**
- `cmd/agent/main.go`: HTTP server main
- `internal/api/handler.go`: `/audit/trigger` route registration
- `internal/agent/agent.go`: Programmatic audit graph entry (`Agent.Run`)

**Configuration:**
- `configs/repos.json`: Repo manifest (`name`, `owner`, `provider`, `default_branch`)
- `Makefile`: `test`, `vet`, `lint`, `fmt`, `build`, `run`, `smoke`
- `Dockerfile`: Build/run image; default `-manifest configs/repos.json`
- `go.mod`: Module path and Go version
- `.gitignore`: Ignores `bin/`, `coverage.out`

**Core Logic:**
- `internal/supervisor/supervisor.go`: Scope planning
- `internal/agent/agent.go`: Parallel execution
- `internal/reposcanner/scanner.go`: Standardization rubric
- `internal/cicd/auditor.go`: CI/CD maturity rubric
- `internal/coverage/analyzer.go`: Coverage risk
- `internal/finops/analyzer.go`: Cloud waste flags
- `internal/aggregator/aggregator.go`: Merge/rank/DORA
- `internal/output/writer.go`: Persist + digest
- `internal/domain/types.go`: Shared types

**Testing:**
- Co-located `*_test.go` beside each package under `internal/`
- Cross-package e2e: `internal/agent/e2e_test.go`, `internal/api/live_trigger_test.go`
- Smoke script: `scripts/test-audit-trigger.sh`

**Documentation:**
- `README.md`: Portfolio overview + mermaid topology
- `TDD.md`: Full technical design (problem, nodes, schema, rubrics)
- `.planning/codebase/`: Generated architecture/structure maps

**Schema:**
- `migrations/001_schema.sql`: `audit_runs`, `repo_scores`, `findings`, `platform_health_current`

## Naming Conventions

**Files:**
- Lowercase single-word primary file matching package role: `agent.go`, `handler.go`, `scanner.go`, `auditor.go`, `analyzer.go`, `writer.go`, `manifest.go`, `types.go`
- Tests: `{file}_test.go` or descriptive `*_test.go` (`e2e_test.go`, `live_trigger_test.go`, `repos_manifest_test.go`)
- SQL: `{NNN}_{name}.sql` under `migrations/`

**Directories:**
- Lowercase, no plurals for packages: `reposcanner`, `cicd`, `finops`, `aggregator`
- Standard Go roots: `cmd/`, `internal/`, `configs/`, `migrations/`, `scripts/`

**Packages / types:**
- Package name = directory name
- Exported types: PascalCase (`AuditState`, `PlanResult`, `Dependencies`)
- Worker IDs in plans: snake_case strings (`repo_scanner`, `cicd_auditor`, `coverage_analyzer`, `cloud_finops`)
- Domain enums: typed string consts (`ScopeFull`, `FindingTypeCICDGap`, `SeverityCritical`)

**Special Patterns:**
- Interfaces defined next to consumer: `api.Runner`, `output.Store`, `output.Notifier`, `agent.Dependencies`, `reposcanner.FileChecker`
- Adapters named by role: `store.Memory`, `demo.Sources`, `logNotifier` / `agentRunner` in main
- No barrel `doc.go` files; package comment on the primary source file

## Where to Add New Code

**New audit worker (new analysis dimension):**
- Primary code: `internal/{workername}/` (e.g. `internal/security/analyzer.go`)
- Types needed by others: Prefer `internal/domain` for shared finding types; keep worker-local Result structs in the worker package
- Wire into plan: Add worker name to `supervisor.Plan` switch in `internal/supervisor/supervisor.go`
- Fan-out: Call from `internal/agent/agent.go` inside `runWorker` with mutex-protected merge
- Tests: `internal/{workername}/{name}_test.go`
- If external I/O needed: Extend `agent.Dependencies` and implement in `internal/demo` (+ future real client package)

**New HTTP endpoint:**
- Definition/handler: `internal/api/handler.go` (register on `mux` in `NewHandler`)
- Tests: `internal/api/handler_test.go`
- Do not put business logic in the handler — call an interface or the agent

**New persistence backend (e.g. Postgres):**
- Implementation: `internal/store/postgres.go` (or similar) implementing `output.Store`
- Wire in: `cmd/agent/main.go` instead of/in addition to `store.Memory`
- Schema: Add/alter files under `migrations/`

**New notifier (real Slack):**
- Implementation: New type satisfying `output.Notifier`
- Wire in: Replace `logNotifier` in `cmd/agent/main.go`

**New manifest field / repo metadata:**
- Type: `domain.RepoConfig` in `internal/domain/types.go`
- Loader: `internal/config/manifest.go`
- Data: `configs/repos.json`
- Tests: `internal/config/manifest_test.go`

**Shared pure helpers:**
- Domain scoring/enums: `internal/domain/`
- Do not create a generic `internal/utils` unless multiple packages need the same helper; prefer keeping helpers package-local

**Utilities:**
- Shared helpers: Prefer `internal/domain` for cross-cutting domain math
- Type definitions: `internal/domain/types.go`
- Test-only fakes: Define in `*_test.go` next to the package under test (see `fakeRunner`, `stubDeps`, `fakeRepoClient`)

## Special Directories

**bin/**
- Purpose: Compiled `agent` binary from `make build` / `go build -o bin/agent`
- Source: Build artifacts
- Committed: No (`.gitignore`)

**coverage.out/**
- Purpose: Coverage profile from `make test`
- Source: `go test -coverprofile=coverage.out`
- Committed: No (`.gitignore`)

**migrations/**
- Purpose: Reference DDL for Postgres persistence described in TDD
- Source: Hand-written SQL
- Committed: Yes — runtime does not apply migrations automatically

**testdata/**
- Purpose: Conventional Go fixture directory
- Source: Reserved; currently empty (tests use inline strings / temp dirs / `configs/repos.json`)
- Committed: Yes (directory may be empty)

**configs/**
- Purpose: Runtime configuration checked into the repo for demos
- Source: Hand-maintained JSON
- Committed: Yes

**.planning/codebase/**
- Purpose: Codebase map documents for GSD planning/execution
- Source: Produced by `/gsd-map-codebase`
- Committed: Yes (when project uses GSD)

---

*Structure analysis: 2026-07-12*
*Update when directory structure changes*
