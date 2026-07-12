# Stack Research

**Domain:** Production Go platform-maturity audit agent (GitHub/GitLab + AWS FinOps → Postgres → Slack/LLM digests)
**Researched:** 2026-07-12
**Confidence:** HIGH

Brownfield baseline: Go 1.26.2 supervisor/workers, `net/http`, `demo.Sources`, `store.Memory`, `uuid` + `yaml.v3` only. This stack hardens that binary — it does not replace the orchestrator.

## Recommended Stack

### Core Technologies

| Technology | Version | Purpose | Why Recommended | Confidence |
|------------|---------|---------|-----------------|------------|
| Go | 1.26.2 (keep `go.mod`) | Runtime / language | Already pinned; matches Docker builder; AWS SDK v2 and pgx v5 require recent Go (1.24+/1.25+) | HIGH |
| `net/http` ServeMux | stdlib | `/audit/trigger`, health, metrics | 3–5 routes; chi/gin add surface with no gain for CronJob + on-demand trigger | HIGH |
| PostgreSQL | 16.x (server) | Durable audit runs, scores, findings, MV | Schema already in `migrations/001_schema.sql`; PG 16 is current stable LTS-class choice for new deploys | HIGH |
| `jackc/pgx/v5` + `pgxpool` | **v5.10.0** | Driver + pool | De-facto Go Postgres driver; native interface + pool for concurrent writers; CGO-free matches static Alpine binary | HIGH |
| AWS SDK for Go v2 | core **v1.42.1**; modules independently versioned | Cost Explorer, CloudWatch, EC2 | Official modular SDK; v1 is legacy — do not introduce it | HIGH |
| `google/go-github` | **v89.0.0** | GitHub REST (repos, contents, branch protection, Actions YAML) | Official client; pagination + rate metadata on every `Response` | HIGH |
| `gitlab.com/gitlab-org/api/client-go/v2` | **v2.46.0** | GitLab REST (files, pipelines, protection) | Official successor to archived `xanzy/go-gitlab`; SemVer v2 path | HIGH |
| Kubernetes CronJob + Deployment | K8s 1.28+ APIs | Weekly full audit + long-lived HTTP for on-demand | Matches approved deploy model; no operator/Helm required for MVP | HIGH |

### Supporting Libraries

| Library | Version | Purpose | When to Use | Confidence |
|---------|---------|---------|-------------|------------|
| `github.com/aws/aws-sdk-go-v2/config` | **v1.32.29** | `LoadDefaultConfig` (IRSA / env / shared creds) | Always for live FinOps | HIGH |
| `.../service/costexplorer` | **v1.66.0** | Spend / waste signals | FinOps worker | HIGH |
| `.../service/cloudwatch` | **v1.63.0** | CPU/utilization for IDLE / OVER_PROVISIONED | FinOps worker | HIGH |
| `.../service/ec2` | **v1.316.0** | Instance inventory + tags → service ownership | FinOps worker | HIGH |
| `.../service/secretsmanager` | **v1.43.0** | Optional secret fetch at boot | When not injecting secrets via K8s Secret/env | MEDIUM |
| `github.com/golang-migrate/migrate/v4` | **v4.19.1** | Apply `migrations/*.sql` | Boot or `make migrate`; numbering already matches `001_*.sql` | HIGH |
| `github.com/slack-go/slack` | **v0.27.0** | Digests / critical alerts | Incoming webhook for digests; bot token + `PostMessage` if threading/blocks needed | HIGH |
| `github.com/anthropics/anthropic-sdk-go` | **v1.57.0** | Executive summary (summarization only) | Default LLM if Anthropic key present | HIGH |
| `github.com/openai/openai-go/v3` | **v3.42.0** | Alternate LLM provider | When `OPENAI_API_KEY` is the org standard; wrap behind one `Summarizer` interface | HIGH |
| `golang.org/x/sync` | **v0.22.0** | `errgroup` + `semaphore` | Bound per-repo fan-out (8–16) under live API rate limits | HIGH |
| `golang.org/x/time` | **v0.15.0** | Token-bucket rate limiter | GitHub/GitLab/AWS/Codecov client wrappers | HIGH |
| `github.com/gofri/go-github-ratelimit/v2` | **v2.0.2** | Respect GitHub secondary rate limits | Live GitHub path (~560 calls / 28 repos) | HIGH |
| `github.com/cenkalti/backoff/v5` | **v5.0.3** | Exponential backoff on 429/5xx | All live HTTP adapters | HIGH |
| `github.com/prometheus/client_golang` | **v1.23.2** | `/metrics` (duration, partial audits, API errors) | K8s scrape; aligns with TDD node timing metrics | HIGH |
| `log/slog` | stdlib | Structured logs with `audit_run_id` | Replace ad-hoc `log.Printf` in production path | HIGH |
| Codecov API v2 | HTTP (`api.codecov.io`) | Coverage fetch | No official Go SDK — thin `net/http` + bearer token behind `Dependencies` | HIGH |
| `github.com/google/uuid` | **v1.6.0** (keep) | Finding / run IDs | Already required; assign IDs before Postgres inserts | HIGH |
| `gopkg.in/yaml.v3` | **v3.0.1** (keep) | CI YAML parse | Already required by cicd auditor | HIGH |

### Development / Test Tools

| Tool | Purpose | Notes | Confidence |
|------|---------|-------|------------|
| `testcontainers-go` + `modules/postgres` | **v0.43.0** | Integration tests for `store/postgres` + migrations | Prefer over shared CI Postgres for hermeticity | HIGH |
| `httptest` / recorded fixtures | Contract tests for GitHub/AWS/Codecov | Do not hit live APIs in unit CI | HIGH |
| `golangci-lint` | **v2.12.2** | Lint gate | Commit `.golangci.yml`; wire into CI | HIGH |
| `govulncheck` | toolchain | Supply-chain scan once SDK surface expands | Add `Makefile` target | HIGH |
| `go test -race` | Concurrency bugs | Keep existing Makefile target | HIGH |

## Installation

```bash
# Persistence
go get github.com/jackc/pgx/v5@v5.10.0
go get github.com/golang-migrate/migrate/v4@v4.19.1
go get github.com/golang-migrate/migrate/v4/database/pgx/v5
go get github.com/golang-migrate/migrate/v4/source/file

# GitHub / GitLab
go get github.com/google/go-github/v89@v89.0.0
go get github.com/gofri/go-github-ratelimit/v2@v2.0.2
go get gitlab.com/gitlab-org/api/client-go/v2@v2.46.0

# AWS FinOps
go get github.com/aws/aws-sdk-go-v2@v1.42.1
go get github.com/aws/aws-sdk-go-v2/config@v1.32.29
go get github.com/aws/aws-sdk-go-v2/service/costexplorer@v1.66.0
go get github.com/aws/aws-sdk-go-v2/service/cloudwatch@v1.63.0
go get github.com/aws/aws-sdk-go-v2/service/ec2@v1.316.0

# Slack + LLM (pick one LLM; keep interface for both)
go get github.com/slack-go/slack@v0.27.0
go get github.com/anthropics/anthropic-sdk-go@v1.57.0
# optional alternate:
go get github.com/openai/openai-go/v3@v3.42.0

# Concurrency / resilience / metrics
go get golang.org/x/sync@v0.22.0
go get golang.org/x/time@v0.15.0
go get github.com/cenkalti/backoff/v5@v5.0.3
go get github.com/prometheus/client_golang@v1.23.2

# Dev
go get github.com/testcontainers/testcontainers-go@v0.43.0
go get github.com/testcontainers/testcontainers-go/modules/postgres@v0.43.0
```

Pin exact versions in `go.mod` via the gets above; re-run `go mod tidy`. AWS service modules version independently — always `go get` each service path, not only the core module.

## Alternatives Considered

| Recommended | Alternative | When to Use Alternative |
|-------------|-------------|-------------------------|
| `pgx/v5` + native API | `database/sql` + `pgx/stdlib` or `lib/pq` | Only if you must share a `*sql.DB` with a library that requires it; prefer native pgx for new `Store` |
| `golang-migrate` | `pressly/goose/v3` (**v3.27.2**) | Prefer goose if you want Go migration funcs + SQL in one tool; rename files to goose convention |
| `go-github` REST | `shurcooL/githubv4` GraphQL | Add GraphQL later to batch file/protection queries when REST call count hits secondary limits |
| Slack Incoming Webhook | Bot token + `chat.postMessage` | Need Block Kit layouts, threads, or multi-channel routing |
| Anthropic SDK | OpenAI Go v3 | Org already standardized on OpenAI keys/models |
| In-process async + CronJob | Redis/SQS + worker fleet | Only if audit duration or multi-replica fan-out exceeds one pod (~28 repos should not) |
| Prometheus metrics | Full OpenTelemetry stack | Need distributed traces across many services; overkill for single binary |
| Stdlib ServeMux | chi / echo / gin | Middleware zoo or dozens of routes; not this service |

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| `github.com/aws/aws-sdk-go` (v1) | Maintenance mode / legacy; different API surface | AWS SDK **v2** modular services |
| `github.com/lib/pq` | CGO-adjacent history; pgx is the modern pure-Go standard | `jackc/pgx/v5` |
| `github.com/xanzy/go-gitlab` | Archived; moved to GitLab.org | `gitlab.com/gitlab-org/api/client-go/v2` |
| GORM / ent / sqlc (for MVP store) | Schema is small append-only CRUD; ORMs obscure transactions and MV refresh | Hand-written pgx queries implementing `output.Store` |
| chi / gin / Fiber | Unnecessary dependency for trigger + health + metrics | stdlib `ServeMux` |
| LangGraph / LangChain Go ports / agent frameworks | Metric collection must stay deterministic; LLM is summarization-only | Thin `Summarizer` interface + official Anthropic/OpenAI SDK |
| Celery / Temporal / Asynq for MVP | Weekly CronJob + `202 Accepted` background run is enough at ~28 repos | In-process job + Postgres run status |
| Python sidecar for LLM | Splits deploy, secrets, and failure domains | Same Go binary, optional env-gated summarizer |
| Cloning full git repos for file checks | Bandwidth + disk; rate-limit hostile | Contents/tree API (GitHub) / repository files API (GitLab) |
| Embedding Slack Bolt / Socket Mode | Digests are outbound-only | Webhook or `PostMessage` |
| Putting secrets in `configs/` or images | CONCERNS + TDD require read-only tokens via env/Secrets Manager | K8s Secret → env; optional Secrets Manager at boot |

## Stack Patterns by Variant

**If `SOURCES=demo` (local/tests):**
- Keep `demo.Sources` + `store.Memory`; no AWS/GitHub/Slack SDKs required at runtime
- Because: deterministic CI and portfolio demos must not need credentials

**If `SOURCES=live` (production CronJob):**
- Wire pgx store, go-github + GitLab client-go, AWS v2 CE/CW/EC2, Codecov HTTP, Slack notifier
- Bound concurrency with `semaphore` + GitHub rate-limit transport
- Because: unbounded fan-out will 429 long before CPU saturates

**If LLM summarization enabled:**
- Call LLM only after aggregator ranks findings; feed structured digest text, not raw API payloads
- Fail open: persist findings + send deterministic Slack digest if LLM errors
- Because: narrative is optional; audit truth must not depend on model availability

**If multi-replica Deployment:**
- Single-flight / lease on `audit_runs` (Postgres advisory lock or `INSERT … ON CONFLICT` for “running” scope)
- Because: two pods both receiving CronJob/HTTP triggers would double API spend

**If GitHub secondary rate limits dominate:**
- Add GraphQL (`githubv4`) for batched tree + protection queries; keep REST for simple file GETs
- Because: TDD’s ~560 REST calls / full audit is the first production cliff

## Version Compatibility

| Package A | Compatible With | Notes |
|-----------|-----------------|-------|
| Go **1.26.2** | pgx **v5.10.0** | pgx supports current Go releases (docs: Go 1.25+); 1.26.2 is fine |
| Go **1.26.2** | AWS SDK v2 modules (Go **≥1.24**) | Cost Explorer / CloudWatch modules require 1.24+ |
| Go **1.26.2** | anthropic-sdk-go **v1.57.0** (Go **≥1.24**) | OK |
| `go-github/v89` | Import path must include `/v89` | Breaking major bumps change import path — pin and upgrade deliberately |
| `client-go/v2` | Import `.../client-go/v2` | Do not mix v1 and v2 module paths |
| `migrate/v4` + `pgx/v5` database driver | pgx **v5.10.0** | Use `database/pgx/v5` migrate driver, not deprecated lib/pq driver |
| `openai-go/v3` vs `openai-go` v1 | Prefer **`/v3`** | v1 module still publishes but v3 is current major |
| Alpine runtime + `CGO_ENABLED=0` | All recommended libs | Pure Go — keep static binary; avoid CGO deps |

## Production Wiring Map (brownfield)

| Concern (existing) | Stack addition | Integration point |
|--------------------|----------------|-------------------|
| `store.Memory` only | pgxpool + migrate | `internal/store/postgres.go` implementing `output.Store` |
| `demo.Sources` | go-github, client-go, AWS v2, Codecov HTTP | New packages behind `agent.Dependencies`; `-sources=demo\|live` |
| `logNotifier` | slack-go webhook/API | `output.Notifier`; digest failure non-fatal |
| No LLM | anthropic or openai-go | Post-aggregate summarizer only |
| Sync HTTP = full audit | Same binary; async job status in Postgres | `202` + `GET /audit/runs/{id}` |
| Unbounded goroutines | `x/sync/semaphore` + rate limiters | `internal/agent` worker spawn |
| No probes/metrics | stdlib `/healthz` + prometheus | `cmd/agent/main.go` mux |
| No schedule | K8s CronJob YAML | Hits authenticated `/audit/trigger` or runs one-shot mode |

## Sources

- [pgx v5.10.0 changelog / pkg.go.dev](https://pkg.go.dev/github.com/jackc/pgx/v5) — pool usage, Go/PG support — **HIGH**
- [google/go-github README](https://github.com/google/go-github) — **v89.0.0** (2026-07-06) — **HIGH**
- [AWS SDK for Go v2 migrate guide](https://docs.aws.amazon.com/sdk-for-go/v2/developer-guide/migrate-gosdk.html) — modular services — **HIGH**
- [proxy.golang.org `@latest`](https://proxy.golang.org/) — verified 2026-07-12 for pgx, go-github, AWS modules, slack-go, client-go/v2, migrate, goose, anthropic, openai-go/v3, prometheus, x/sync, x/time, testcontainers, backoff, go-github-ratelimit — **HIGH**
- [GitLab client-go](https://gitlab.com/gitlab-org/api/client-go) — official v2; xanzy archived — **HIGH**
- [Codecov API overview](https://docs.codecov.com/reference/overview) — bearer token HTTP, no Go SDK — **HIGH**
- [slack-go/slack](https://github.com/slack-go/slack) — **v0.27.0** Web API + webhooks — **HIGH**
- [anthropic-sdk-go](https://pkg.go.dev/github.com/anthropics/anthropic-sdk-go) — **v1.57.0** — **HIGH**
- [openai-go/v3](https://pkg.go.dev/github.com/openai/openai-go/v3) — **v3.42.0** — **HIGH**
- Project `.planning/codebase/STACK.md` + `CONCERNS.md` + `go.mod` — brownfield constraints — **HIGH**

### Gaps / watch items

- Exact AWS module patch versions move weekly — re-`go get` at implementation time; treat table versions as 2026-07-12 pins (**MEDIUM** drift risk).
- `shurcooL/githubv4` remains pseudo-version only — acceptable for optional GraphQL phase; no semver tag (**MEDIUM**).
- Slack-go has no major version — minor bumps can break; pin tightly and read changelog on upgrade (**MEDIUM**).

---
*Stack research for: production Go audit agent (GitHub/GitLab + AWS FinOps + Postgres + Slack/LLM)*
*Researched: 2026-07-12*
