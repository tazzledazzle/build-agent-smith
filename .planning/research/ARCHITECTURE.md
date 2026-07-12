# Architecture Research

**Domain:** Production-ready Go platform maturity audit agents (brownfield)
**Researched:** 2026-07-12
**Confidence:** HIGH

## Standard Architecture

### System Overview

Extend the existing single-binary topology; do not introduce a second orchestrator or message bus for MVP production parity.

```
┌──────────────────────────────────────────────────────────────────────────┐
│                     Transport / Ops Layer                                 │
│  Auth middleware · POST /audit/trigger (async 202) · GET /audit/runs/{id}│
│  GET /healthz · GET /metrics · CronJob HTTP client                        │
└────────────────────────────────┬─────────────────────────────────────────┘
                                 │ enqueue job (audit_run_id)
                                 ▼
┌──────────────────────────────────────────────────────────────────────────┐
│                     Job Runner (process-local)                            │
│  Detached ctx · single-flight / queue · status: PENDING→RUNNING→DONE     │
└────────────────────────────────┬─────────────────────────────────────────┘
                                 │
                                 ▼
┌──────────────────────────────────────────────────────────────────────────┐
│                     Orchestration (unchanged shape)                       │
│  supervisor.Plan → agent.Run (bounded pool) → aggregator.Aggregate        │
├──────────────┬──────────────┬──────────────┬─────────────────────────────┤
│ Repo Scanner │ CI/CD Auditor│ Coverage     │ Cloud FinOps                 │
└──────┬───────┴──────┬───────┴──────┬───────┴──────────────┬──────────────┘
       │              │              │                       │
       └──────────────┴──────┬───────┴───────────────────────┘
                             ▼
┌──────────────────────────────────────────────────────────────────────────┐
│  Narrative (optional node)  →  Output Writer                              │
│  LLM summarizer (post-aggregate only) → Store + Notifier                  │
├─────────────────────────────┬────────────────────────────────────────────┤
│ Postgres (output.Store)     │ Slack / log Notifier                        │
│ Memory (tests/demo)         │                                             │
└─────────────────────────────┴────────────────────────────────────────────┘
                             ▲
                             │
┌────────────────────────────┴─────────────────────────────────────────────┐
│  Live / Demo Adapters implementing agent.Dependencies                     │
│  GitHub·GitLab (rate.Limiter) · Codecov · AWS CE/CW/EC2/RDS               │
└──────────────────────────────────────────────────────────────────────────┘
```

### Component Responsibilities

| Component | Responsibility | Typical Implementation |
|-----------|----------------|------------------------|
| API + Auth | Validate trigger, authenticate, return 202 + run id | `internal/api` + shared-secret middleware |
| Job runner | Own audit lifecycle off the HTTP request | `cmd/agent` or `internal/jobs` goroutine + status store |
| Supervisor | Scope → workers/repos + mint `audit_run_id` | Existing `internal/supervisor` (keep pure) |
| Agent | Bounded parallel fan-out; merge findings/errors | Existing `internal/agent` + `errgroup.SetLimit` |
| Workers | Deterministic analysis only | Existing `reposcanner`/`cicd`/`coverage`/`finops` |
| Dependencies | External I/O ports | Keep interface; add `demo` + `live` adapters |
| Aggregator | Dedupe, rank, DORA/repo scores | Existing `internal/aggregator` (fix scoring bugs here) |
| Summarizer | Executive narrative from ranked findings | New `output.Summarizer` / `internal/llm` — never drives metrics |
| Output Writer | Persist run/scores/findings; notify | Existing `output.Writer` + Postgres/Slack adapters |
| Store | Durable audit history | `store.Postgres` implements `output.Store`; `Memory` for tests |
| Rate limiters | Protect upstream quotas | `golang.org/x/time/rate` inside live HTTP clients |
| Deploy | Scheduled + on-demand | Docker image + K8s CronJob hitting authenticated trigger |

## Recommended Project Structure

```
cmd/agent/main.go                 # Flags: addr, manifest, sources, store, auth
internal/
  api/                            # HTTP: trigger, status, health; auth middleware
  jobs/                           # Async run enqueue + status machine (new)
  agent/                          # Orchestrator + Dependencies ports
  supervisor/                     # Scope planner (unchanged contract)
  reposcanner|cicd|coverage|finops/
  aggregator/
  output/                         # Writer, Store, Notifier, Summarizer interfaces
  store/
    memory.go
    postgres.go                   # New — migrations/001_schema.sql
  clients/                        # New — live adapters only
    github/
    gitlab/
    codecov/
    awsfinops/
    slack/
  llm/                            # New — summarization client (OpenAI/Anthropic)
  demo/                           # Fixture Dependencies (keep)
  domain/                         # Shared types; add run status / summary fields
  config/
migrations/
configs/repos.json
deploy/                           # Optional: CronJob YAML, ServiceAccount
```

### Structure Rationale

- **`internal/clients/*`:** Keep live SDK code out of workers and out of `agent` so analysis stays pure and testable with demo fixtures.
- **`internal/jobs`:** Isolate async lifecycle (PENDING/RUNNING/COMPLETE/FAILED) from `api.Handler` so HTTP stays thin and request context is not reused for background work.
- **`internal/llm`:** Separate package enforcing “summarization only”; workers must never import it.
- **`store` beside `output`:** Persist adapters implement `output.Store` — same seam already used by Memory.
- **No Celery/Redis for MVP:** Process-local queue + Postgres status is enough for single-replica CronJob + on-demand; add a broker only if multi-replica concurrency becomes a requirement.

## Architectural Patterns

### Pattern 1: Ports & Adapters at the Edges

**What:** External systems sit behind small interfaces defined next to consumers (`agent.Dependencies`, `output.Store`, `output.Notifier`, future `Summarizer`).
**When to use:** Every live integration (GitHub, AWS, Slack, Postgres, LLM).
**Trade-offs:** Slight wiring boilerplate in `main`; huge win for demo/live dual-mode and unit tests.

**Example:**
```go
// cmd/agent/main.go — select adapters, not behavior
deps := demo.Sources{}
if *sources == "live" {
    deps = clients.NewLive(cfg) // rate-limited GitHub/Codecov/AWS
}
store := store.NewMemory()
if dsn := os.Getenv("DATABASE_URL"); dsn != "" {
    store = store.NewPostgres(ctx, dsn)
}
```

### Pattern 2: Async Trigger with Durable Run Record

**What:** Auth’d `POST /audit/trigger` inserts/updates `audit_runs` as `PENDING`, returns `202` + `audit_run_id`, then a job runner executes `Agent.Run` → `Writer.Write` on a detached context.
**When to use:** Full audits (≥ minutes) and any CronJob/on-demand path that must survive LB timeouts.
**Trade-offs:** Need status polling (`GET /audit/runs/{id}`) and careful shutdown draining; incremental &lt;90s could stay sync later, but one async path is simpler.

**Example:**
```go
// Handler returns immediately; do NOT pass r.Context() into the job.
runID := uuid.NewString()
_ = jobs.Enqueue(ctx, Job{ID: runID, Scope: scope, Target: repo})
w.WriteHeader(http.StatusAccepted)
json.NewEncoder(w).Encode(TriggerResponse{AuditRunID: runID, Status: "PENDING"})
```

### Pattern 3: Bounded Fan-out + Client-Side Rate Limits

**What:** Cap in-flight workers with `errgroup.SetLimit(N)`; throttle outbound calls with `rate.Limiter` inside live clients. Two layers solve two problems: CPU/FD pressure vs upstream 429s.
**When to use:** Live mode at ~28 repos (~560 GitHub + ~280 AWS calls per TDD).
**Trade-offs:** Slightly longer wall time under tight limits; tune N to quota, not CPU count.

**Example:**
```go
g, ctx := errgroup.WithContext(ctx)
g.SetLimit(8) // repo×worker concurrency cap
g.Go(func() error { return runOne(ctx, task) })
```

### Pattern 4: Post-Aggregate LLM Summarization Only

**What:** After `aggregator.Aggregate`, optional `Summarizer.Summarize(ctx, state)` produces narrative text for Slack/dashboard. Metrics, scores, and findings remain deterministic.
**When to use:** Leadership digests; never for scope planning or rubric scoring.
**Trade-offs:** Extra latency/cost (~50K tokens/run per TDD); must degrade gracefully (skip summary, keep findings) if LLM fails.

## Data Flow

### Request Flow (production trigger)

```
CronJob / operator
    ↓  Authorization: Bearer|X-Audit-Token
POST /audit/trigger {scope, repo?}
    ↓
Auth middleware → validate body → jobs.Enqueue
    ↓ 202 {audit_run_id, status: PENDING}
Job runner (detached ctx + audit timeout)
    ↓
supervisor.Plan → agent.Run (bounded pool)
    ↓ deps calls (rate-limited live clients | demo)
Workers → findings/metrics (+ AuditError → PARTIAL_AUDIT)
    ↓
aggregator.Aggregate (dedupe, priority, DORA)
    ↓
llm.Summarize (optional) → state.ExecutiveSummary
    ↓
output.Writer → Postgres Save* → Slack PostDigest
    ↓
audit_runs.status = COMPLETE | PARTIAL_AUDIT | FAILED
GET /audit/runs/{id} ← poll/status consumers
```

### State Management

```
domain.AuditState (per run, in-memory during orchestration)
    ↓ Writer
Postgres: audit_runs / repo_scores / findings
    ↓ REFRESH
platform_health_current (materialized view)
```

- No shared mutable agent state across requests beyond Store + JobRunner.
- Background jobs must copy needed values from the request; never use `r.Context()` after 202.
- Demo `store.Memory` remains for tests; production always Postgres when `DATABASE_URL` set.

### Key Data Flows

1. **Scheduled full audit:** CronJob → auth’d trigger → async job → all workers → Postgres + Slack (&lt;15 min SLO).
2. **FinOps-only:** Same path, supervisor routes only `cloud_finops` (&lt;3 min).
3. **Incremental:** Webhook/API with `scope=incremental` + `repo` → three repo workers, no FinOps (&lt;90s); webhook HMAC lands after shared-secret auth.
4. **Status poll:** Clients read Postgres/`jobs` status by `audit_run_id` without re-running workers.
5. **LLM narrative:** Ranked findings in → short executive text out → attached to digest only.

## Suggested Build Order

Aligned with approved track order **ops → live → narrative**. Fine granularity for roadmap phases (10 steps):

| # | Phase | Integrates | Depends on | Avoids |
|---|-------|------------|------------|--------|
| 1 | **Finding identity + Store contract hardening** | UUID IDs on findings; FinOps `RepoName` via tags; Writer transactional save | — | Postgres insert failures |
| 2 | **Postgres store** | `store.Postgres` + migrate `001_schema.sql`; DSN wiring; keep Memory | Phase 1 | Data loss on restart |
| 3 | **Auth on trigger** | Shared-secret header middleware; opaque 500s | — | Open audit endpoint before live APIs |
| 4 | **Async jobs + status API** | `202` enqueue, detached ctx, `GET /audit/runs/{id}`, PENDING→… | Phases 2–3 | LB/client timeouts on full audit |
| 5 | **Bounded concurrency + health/metrics** | `errgroup.SetLimit`; `/healthz`; basic run metrics | Phase 4 | Unbounded goroutine storms |
| 6 | **Live source factory** | `-sources=demo\|live` wiring; secrets via env/Secrets Manager pattern | Phase 5 | Accidental live calls in tests |
| 7 | **Live SCM + rate limits** | GitHub/GitLab clients implementing `Dependencies` fetch/HasFile; `rate.Limiter` | Phase 6 | 429 storms / quota burn |
| 8 | **Live coverage + FinOps adapters** | Codecov/parsers path; AWS CE/CW/EC2/RDS inventory | Phase 7 | Incomplete live audit loop |
| 9 | **Slack notifier + scoring fixes** | Real `Notifier`; DORA unknown-safe; non-zero priority for non-FinOps; branch protection checks | Phases 2, 8 | Fake-healthy leadership metrics |
| 10 | **LLM summary + CronJob deploy + ~28-repo scale** | Summarizer node; K8s CronJob; manifest scale/load validation | Phases 4–9 | Narrative without trusted data/ops |

**Ordering rationale:** Persist and auth before live I/O; async and pools before multi-repo live fan-out; scoring/LLM only after real findings exist; deploy last once the HTTP contract is production-shaped.

**Research flags:**
- Phase 4: Confirm single-replica job semantics (in-process queue vs Postgres `SKIP LOCKED`) before multi-pod.
- Phase 7–8: Validate GitHub GraphQL batching vs REST for file presence (capacity).
- Phase 10: Confirm OpenAI vs Anthropic SDK choice at implement time; keep behind `Summarizer` interface.

## Scaling Considerations

| Scale | Architecture Adjustments |
|-------|--------------------------|
| 5 demo repos / local | Memory store, sync or async OK, demo sources, no LLM |
| ~28 repos / weekly CronJob | Postgres + async trigger + limit 8–16 workers + client rate limits + Slack |
| Multi-pod / frequent triggers | Postgres advisory lock or `FOR UPDATE SKIP LOCKED` job claim; optional queue |

### Scaling Priorities

1. **First bottleneck:** Unbounded goroutines × live API latency → 429s and pod CPU — fix with `SetLimit` + client limiters (Phase 5–7).
2. **Second bottleneck:** Sync HTTP = full audit duration → client timeouts — fix with async jobs (Phase 4).
3. **Third bottleneck:** Multi-replica duplicate runs — add DB-backed single-flight only when replicas &gt; 1.

## Anti-Patterns

### Anti-Pattern 1: LLM Inside Workers or Supervisor

**What people do:** Ask the model to decide scope, scores, or which APIs to call.
**Why it's wrong:** Non-deterministic audits, cost blowups, unverifiable leadership metrics.
**Do this instead:** Deterministic workers + aggregator; LLM only after ranked findings exist.

### Anti-Pattern 2: Reusing Request Context for Background Audits

**What people do:** `go agent.Run(r.Context(), …)` after writing 202.
**Why it's wrong:** Context cancels when the handler returns; audits abort randomly.
**Do this instead:** `context.WithTimeout(context.WithoutCancel(parent), auditSLO)` (or explicit background + timeout).

### Anti-Pattern 3: Live SDKs Imported by Worker Packages

**What people do:** `cicd` or `finops` calling GitHub/AWS SDKs directly.
**Why it's wrong:** Breaks demo tests, creates import cycles, couples rubrics to vendors.
**Do this instead:** Fetch via `agent.Dependencies`; analyze pure inputs in workers.

### Anti-Pattern 4: Postgres Without Finding IDs / Sync Writer Assumptions

**What people do:** Wire `store.Postgres` while `Finding.ID` is empty and HTTP still blocks on full runs.
**Why it's wrong:** Insert failures; timeouts before persistence completes.
**Do this instead:** Phase 1 IDs → Phase 2 Postgres → Phase 4 async before load testing live.

### Anti-Pattern 5: Unbounded Fan-out “Because WaitGroup Is Easy”

**What people do:** One goroutine per repo×worker with no limit.
**Why it's wrong:** At 28 repos, ~84+ concurrent upstream calls; rate limits and OOM risk.
**Do this instead:** `errgroup.SetLimit` sized to quota; rate limit inside clients.

## Integration Points

### External Services

| Service | Integration Pattern | Notes |
|---------|---------------------|-------|
| GitHub / GitLab | `Dependencies` live client + read-only token | Rate limit; prefer tree/contents batch for scanner |
| Codecov / CI artifacts | `FetchCoverage` → existing parsers | Wire JaCoCo/JSON parsers on live path |
| AWS CE / CW / EC2 / RDS | `FetchCloudInventory` adapter | IAM ReadOnly + Cost Explorer; map `service` tag → repo |
| PostgreSQL | `output.Store` | Schema already in `migrations/001_schema.sql`; refresh matview after write |
| Slack | `output.Notifier` | Make digest failure non-fatal after persist |
| OpenAI / Anthropic | `Summarizer` | Env API key; summarization only; skip on error |
| K8s CronJob | HTTP client → `/audit/trigger` | Shared secret; rely on async 202 |

### Internal Boundaries

| Boundary | Communication | Notes |
|----------|---------------|-------|
| `api` ↔ `jobs` | Enqueue + run id | API never imports workers |
| `jobs` ↔ `agent` | `Agent.Run` | Detached context + timeout |
| `agent` ↔ workers | Direct calls | Workers depend on `domain` only |
| `agent` ↔ clients | `Dependencies` | demo \| live selected in `main` |
| `agent` ↔ aggregator | In-process | After WaitGroup/errgroup |
| aggregator ↔ llm | Summarizer interface | Optional; after scores exist |
| Writer ↔ store/notifier | Interfaces | Postgres/Slack adapters |

## Sources

- Codebase map: `.planning/codebase/ARCHITECTURE.md`, `STRUCTURE.md`, `CONCERNS.md` (2026-07-12) — HIGH
- Project intent: `.planning/PROJECT.md` (ops → live → narrative) — HIGH
- Design reference: `TDD.md` (node topology, schema, capacity, LLM-as-summary) — HIGH for intent; MEDIUM where LangGraph/Python narrative diverges from Go runtime
- Runtime seams: `internal/agent.Dependencies`, `output.Store`/`Notifier`, `api.Runner`, `migrations/001_schema.sql` — HIGH
- Go concurrency practice: `errgroup.SetLimit` + `golang.org/x/time/rate` in outbound clients — MEDIUM (ecosystem consensus; tune limits empirically)

---
*Architecture research for: Go platform maturity audit agents (production integration)*
*Researched: 2026-07-12*
