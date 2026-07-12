# Project Research Summary

**Project:** Platform Maturity Audit Agent — Production Ready
**Domain:** Production Go platform-maturity audit agent (GitHub/GitLab + AWS FinOps → Postgres → Slack/LLM digests)
**Researched:** 2026-07-12
**Confidence:** HIGH

## Executive Summary

This is a brownfield Go audit agent that already ships the TDD node topology (supervisor → parallel workers → aggregator → output) against demo fixtures and an in-memory store. Experts build production parity by hardening the existing single binary—not rewriting in LangGraph/Python or splitting into SEI + FinOps products. The differentiator vs LinearB/CloudHealth is one scheduled agent that emits a unified, ranked findings list from live SCM, coverage, and AWS waste signals, with deterministic metrics and optional LLM narrative only after aggregation.

Recommended approach follows the approved order **ops → live → narrative**: (1) auth, async `202` jobs, Postgres + finding IDs, health/metrics, and bounded concurrency; (2) `demo|live` adapters for GitHub/GitLab, Codecov, AWS FinOps, plus Slack digests and secrets via env/IRSA; (3) honest DORA/priority fixes, branch protection, LLM summarization, ~28-repo scale, and K8s CronJob deploy. Keep stdlib `ServeMux`, pgx v5, AWS SDK v2, official SCM clients, and thin ports (`Dependencies` / `Store` / `Notifier` / `Summarizer`) so tests stay credential-free.

Key risks are trust-destroying fake-healthy DORA (~7.5 from zero metrics), sync HTTP timeouts on full audits, unauthenticated live triggers, unbounded fan-out → GitHub/AWS 429s, empty finding UUIDs blocking Postgres, priority-0 burial of non-FinOps work, and ungrounded LLM prose. Mitigate by fixing scoring/IDs and async/auth before live I/O, rate-limiting clients, transactional persist with non-fatal Slack, confidence-tiered FinOps, and post-validate LLM claims against ranked finding IDs.

## Key Findings

### Recommended Stack

Harden the pinned Go 1.26.2 binary with production adapters; do not replace the orchestrator. Prefer stdlib HTTP (3–5 routes), hand-written pgx queries over ORMs, and official modular SDKs. Dual-mode (`SOURCES=demo|live`) is mandatory so CI stays deterministic.

**Core technologies:**
- **Go 1.26.2 + `net/http` ServeMux** — runtime and trigger/health/metrics routes — already pinned; chi/gin add no value for CronJob + on-demand API
- **PostgreSQL 16 + `jackc/pgx/v5` + golang-migrate** — durable runs/scores/findings + MV refresh — schema exists in `migrations/001_schema.sql`
- **AWS SDK for Go v2** (Cost Explorer, CloudWatch, EC2) — live FinOps inventory/spend — never introduce AWS SDK v1
- **`google/go-github` v89 + GitLab `client-go/v2`** — live SCM/CI/protection APIs — official clients; rate-limit transport required
- **slack-go + Anthropic or OpenAI SDK** — digests + executive summary — webhook/PostMessage; LLM behind `Summarizer` only
- **`x/sync` semaphore + `x/time` rate + backoff** — bound fan-out and respect 429s — first production cliff at ~28 repos
- **K8s CronJob + Deployment** — weekly full audit + long-lived HTTP — matches approved deploy model
- **Prometheus client + `log/slog`** — ops metrics and `audit_run_id` correlation — K8s scrape / SLO debugging

Details: [STACK.md](./STACK.md)

### Expected Features

Table stakes for this milestone equal **full TDD parity**, not a greenfield MVP subset. Ship trust/ops foundations before live adapters; ship honest scores before leadership narrative.

**Must have (table stakes):**
- Authenticated `/audit/trigger` (shared secret) + async jobs (`202` + status poll)
- Postgres persistence + MV refresh; health/ready + structured metrics
- Bounded concurrency + client rate limits; finding UUIDs; write/notify decoupling
- Live GitHub/GitLab, AWS FinOps, Codecov; Slack digests; secrets via env/SM
- Honest DORA (unknown ≠ perfect) + non-zero priority for non-FinOps findings
- Branch protection / required reviewers; `demo|live` dual mode; ~28-repo path
- K8s CronJob + on-demand HTTP; append-only findings + PARTIAL_AUDIT semantics

**Should have (competitive):**
- Unified CI/CD + coverage + FinOps + DORA ranked list (defend by shipping live + honest scores)
- Cost × severity/effort ranking with visible deterministic formula
- LLM executive summary (summarization only, ~50K tokens/run budget)
- FinOps ownership tags (`team`/`service`); Cron autonomy closing the human-visible loop

**Defer (v2+ / anti-features):**
- Webhook incremental + PR score-delta (P2 after async + HMAC + live GitHub)
- GraphQL/ETag GitHub batching (P2 if REST hits secondary limits)
- Auto-remediate PRs, agentic LLM collection, multi-cloud FinOps, APM replacement, DevEx surveys — out of scope

Details: [FEATURES.md](./FEATURES.md)

### Architecture Approach

Extend the existing single-binary topology; no message bus or second orchestrator for MVP. Transport (auth + async API) → process-local job runner → unchanged supervisor/agent/workers → aggregator → optional LLM summarizer → Writer (Postgres + Slack). Live SDKs live in `internal/clients/*` behind `agent.Dependencies`; never import SDKs into workers.

**Major components:**
1. **API + Auth + Jobs** — validate trigger, return 202, detached-context lifecycle (`PENDING→RUNNING→COMPLETE|PARTIAL|FAILED`)
2. **Supervisor / Agent / Workers** — scope planning, bounded fan-out, deterministic analysis only
3. **Live / Demo adapters** — rate-limited GitHub/GitLab/Codecov/AWS vs fixtures
4. **Aggregator + Summarizer** — rank/DORA fixes; LLM post-aggregate narrative only
5. **Output Writer + Store** — transactional Postgres (`Memory` for tests); Slack notifier non-fatal
6. **Deploy** — Docker + K8s CronJob hitting authenticated trigger (`concurrencyPolicy: Forbid`)

Details: [ARCHITECTURE.md](./ARCHITECTURE.md)

### Critical Pitfalls

1. **Fake-healthy DORA from unset zeros (~7.5 composite)** — use unknown/nullable; omit from digests until observations exist; assert in tests before LLM/Slack narrative
2. **Sync HTTP = full audit duration** — always `202` + poll; never pass `r.Context()` into background jobs
3. **Unauthenticated / over-scoped live credentials** — shared-secret auth before live mode; read-only SCM/IAM; opaque client errors
4. **Unbounded fan-out → 429 / PARTIAL_AUDIT** — `errgroup.SetLimit(8–16)` + client rate limiter + Retry-After; prefer tree/contents batch over nested `HasFile`
5. **Empty finding IDs + notifier coupled to persist** — UUID before insert; transactional `SaveAudit`; Slack failure must not fail the run
6. **Priority-0 burial of non-FinOps + ungrounded LLM** — composite rank with severity baselines; summarize ranked top-N JSON only with claim validation

Details: [PITFALLS.md](./PITFALLS.md)

## Implications for Roadmap

Based on research, suggested phase structure maps to the three approved tracks, with architecture’s finer 10-step order collapsed into roadmap-sized phases:

### Phase 1: Trust & Ops Foundation
**Rationale:** Safe live calls and durable history require auth, async jobs, Postgres, IDs, and concurrency caps first — demo path hides all of these failures.
**Delivers:** Shared-secret auth; `202` + `GET /audit/runs/{id}`; `store.Postgres` + migrations + MV refresh; finding UUIDs; transactional save; Slack errors non-fatal; `/healthz`/`/readyz`; run metrics/`audit_run_id` logs; `errgroup` concurrency limit; server timeouts; docs parity (Go orchestrator, not LangGraph-as-runtime).
**Addresses:** Auth, async jobs, Postgres, health/metrics, finding IDs, write/notify decoupling, bounded concurrency skeleton, demo|live wiring prep.
**Avoids:** Sync timeout retries; open trigger abuse; empty-UUID insert failures; notifier marking successful audits failed; unbounded goroutine storms once live lands.

### Phase 2: Live Audit Loop
**Rationale:** Leadership value requires org truth, not fixtures; adapters need the Phase 1 ops skeleton or they burn quota and hang.
**Delivers:** `-sources=demo|live` factory; rate-limited GitHub/GitLab clients; live Codecov → existing parsers; AWS CE/CW/EC2 FinOps with tag→repo mapping; Slack digests; secrets via env/Secrets Manager/IRSA; confidence tiers for waste findings.
**Uses:** go-github, GitLab client-go, AWS SDK v2, Codecov HTTP, slack-go, `x/time` rate + backoff, go-github-ratelimit.
**Implements:** `internal/clients/*` behind `Dependencies`; real `Notifier`; PARTIAL_AUDIT with per-repo errors.
**Avoids:** 429 storms; Cost Explorer-as-utilization fiction; avg-CPU false IDLE dollars; demo sources silently in prod.

### Phase 3: Leadership Narrative & Deploy
**Rationale:** Narrative amplifies whatever scores exist — fix DORA/priority and land branch protection before LLM/CronJob claim production readiness.
**Delivers:** Unknown-safe DORA; non-zero priority for CI/coverage/standardization; branch protection + required reviewers; LLM executive summary (fail-open); ~28-repo manifest/load validation; K8s CronJob + on-demand HTTP manifests; README aligned to real capabilities.
**Addresses:** Honest composites, ranking fixes, LLM summarization, scale path, Cron autonomy.
**Avoids:** Fake-green digests; FinOps-only top-10; hallucinated summaries; overlapping CronJob runs; capacity claims against 5 fixture repos.

### Phase Ordering Rationale

- **Ops before live:** Auth + async + Postgres + rate-limit skeleton prevent unsafe/unobservable live runs (FEATURES dependency graph + PITFALLS Phase 1 mapping).
- **Live before narrative:** Honest ranking/DORA and real findings must exist before Slack/LLM amplify them.
- **Architecture grouping:** Persist/auth → async/pools → source factory → SCM → coverage/FinOps → Slack/scoring → LLM/CronJob/scale matches ports-and-adapters seams and anti-patterns (no LLM in workers; no SDKs in analysis packages).
- **Anti-features stay out:** No auto-remediate, agentic LLM collection, multi-cloud, or Temporal/Redis for MVP (~28 repos fit one pod).

### Research Flags

Phases likely needing deeper research during planning:
- **Phase 1 (async jobs):** Confirm single-replica in-process queue vs Postgres `SKIP LOCKED` / advisory lock before multi-pod.
- **Phase 2 (SCM capacity):** Validate REST call budget (~560/audit) vs GraphQL batching + ETag caching when secondary limits bite.
- **Phase 2 (FinOps signals):** Confirm CloudWatch peak/network vs Compute Optimizer idle APIs for confidence tiers.
- **Phase 3 (LLM):** Confirm OpenAI vs Anthropic at implement time; keep behind `Summarizer` interface.

Phases with standard patterns (skip research-phase):
- **Phase 1 Postgres store / migrate / healthz / shared-secret auth:** Well-documented Go patterns; schema already written.
- **Phase 1 finding UUIDs + Writer transaction semantics:** Clear codebase concern; testcontainers pattern known.
- **Phase 3 CronJob YAML + Docker (existing image):** Standard K8s CronJob + `Forbid` concurrency.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Official clients + proxy.golang.org pins 2026-07-12; brownfield `go.mod` constraints clear |
| Features | HIGH | PROJECT.md + TDD.md + CONCERNS.md scope; SEI/FinOps landscape for boundaries |
| Architecture | HIGH | Existing seams (`Dependencies`, `Store`, `Notifier`) + approved ops→live→narrative order |
| Pitfalls | HIGH | Codebase CONCERNS + GitHub rate-limit docs + DORA/LLM faithfulness literature |

**Overall confidence:** HIGH

### Gaps to Address

- **AWS module patch drift:** Re-`go get` at implementation; treat STACK table as 2026-07-12 pins (MEDIUM).
- **Multi-replica job claiming:** Not required for single CronJob pod; design `audit_runs` lease if Deployment replicas > 1.
- **DORA input sources:** “Unknown-safe” scoring is clear; org definitions of deploy/failure/restore still need product validation before composites appear in digests.
- **GitHub GraphQL:** Optional P2; `shurcooL/githubv4` is pseudo-version only — acceptable if REST becomes the cliff.
- **Slack-go minor bumps:** Pin tightly; no major SemVer — review changelog on upgrade.

## Sources

### Primary (HIGH confidence)

- Project: `.planning/PROJECT.md`, `TDD.md`, `.planning/codebase/{ARCHITECTURE,STRUCTURE,CONCERNS,STACK}.md` (2026-07-12)
- [pgx v5](https://pkg.go.dev/github.com/jackc/pgx/v5), [google/go-github](https://github.com/google/go-github), [AWS SDK Go v2 migrate](https://docs.aws.amazon.com/sdk-for-go/v2/developer-guide/migrate-gosdk.html)
- [GitLab client-go](https://gitlab.com/gitlab-org/api/client-go), [Codecov API](https://docs.codecov.com/reference/overview), [slack-go](https://github.com/slack-go/slack)
- [GitHub REST best practices / rate limits](https://docs.github.com/en/rest/using-the-rest-api/best-practices-for-using-the-rest-api)
- [K8s CronJob concurrencyPolicy](https://kubernetes.io/docs/concepts/workloads/controllers/cron-jobs/)
- proxy.golang.org `@latest` version pins verified 2026-07-12

### Secondary (MEDIUM confidence)

- AWS Compute Optimizer / FinOps rightsizing false-positive practices (P95 vs burst workloads)
- DORA missing-data / gaming implementation articles
- LLM summarization faithfulness literature (grounding / claim validation)
- Go `errgroup.SetLimit` + `x/time/rate` ecosystem consensus (tune empirically at 28 repos)

### Tertiary (LOW confidence)

- Exact secondary-limit behavior under org-specific GitHub Apps vs PATs — validate in staging load test
- Whether GraphQL is required at first 28-repo cutover vs after first PARTIAL_AUDIT spike

---
*Research completed: 2026-07-12*
*Ready for roadmap: yes*
