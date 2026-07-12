# Platform Maturity Audit Agent — Production Ready

## What This Is

A Go platform maturity audit agent that measures infrastructure standardization, CI/CD maturity, test coverage, and cloud FinOps waste across an engineering org’s repositories — then ranks actionable findings for leadership and FinOps. Today it ships as a working local/demo binary (`POST /audit/trigger`, supervisor → parallel workers → aggregator → output). This milestone takes it to **full TDD parity**: live integrations, production ops, and leadership-grade narrative outputs.

## Core Value

Engineering leadership can trust a scheduled audit run to produce accurate, persisted, ranked findings from live GitHub/GitLab + AWS data — without manual audit effort.

## Requirements

### Validated

- ✓ Supervisor plans audit scope (`full` / `incremental` / `finops_only`) and routes workers — existing
- ✓ Parallel worker orchestration (repo scanner, CI/CD auditor, coverage analyzer, cloud FinOps) — existing
- ✓ CI/CD maturity rubric (0–10) with secret-hygiene heuristics — existing
- ✓ Repo standardization file-presence rubric (0–5) — existing
- ✓ Coverage threshold + change-risk scoring — existing
- ✓ FinOps flags (IDLE / OVER_PROVISIONED / ORPHANED / UNTAGGED) — existing
- ✓ Findings aggregator (dedupe, priority rank, DORA composite placeholders) — existing
- ✓ Output writer interface + in-memory store + log digest — existing
- ✓ HTTP `POST /audit/trigger` with scope validation — existing
- ✓ Demo data sources and smoke/e2e test coverage — existing
- ✓ Postgres schema defined in `migrations/001_schema.sql` (not wired) — existing
- ✓ Docker image + Makefile build/run/test/smoke — existing

### Active

- [ ] Production trust & ops: Postgres persistence, auth on trigger, async audit jobs, health/metrics, bounded concurrency and rate limits
- [ ] Live audit loop: real GitHub/GitLab + AWS FinOps + Codecov adapters (`demo|live`), Slack digests, secrets via env/Secrets Manager pattern
- [ ] Leadership narrative: fix DORA/priority scoring bugs, branch protection / required reviewers, LLM executive summary (summarization only), scale toward ~28 repos
- [ ] Deploy as containerized K8s CronJob (+ on-demand HTTP) per TDD operational model
- [ ] Align docs with Go orchestrator reality (LangGraph narrative → historical/appendix)

### Out of Scope

- Auto-remediate findings / open remediation PRs — TDD non-goal; humans act
- Replace APM (Datadog/Grafana) for runtime observability — separate concern
- Application-layer code quality audits — separate concern
- External vendor/SaaS dependency audits — TDD non-goal
- Rewriting the system in Python/LangGraph — keep Go supervisor/worker architecture
- Full agentic LLM control of metric collection — LLM for summarization/prioritization narrative only

## Context

**Brownfield baseline:** Portfolio implementation matches the TDD node topology in Go, but wires `demo.Sources{}` and `store.Memory`. Codebase map (`.planning/codebase/`, especially `CONCERNS.md`) catalogs the production gaps: unauthenticated trigger, sync HTTP = full audit duration, fake-healthy DORA from zero metrics, empty finding IDs, priority-0 non-FinOps findings, no live clients.

**Milestone intent:** User chose **full TDD parity** and explicitly included all three value tracks:
1. Trust & ops first (foundation)
2. Live audit loop (real data)
3. Leadership narrative (scores + LLM summary + scale)

**Agreed build order:**
1. Trust & ops → 2. Live audit loop → 3. Leadership narrative

**Defaults approved:**
- Deploy: Docker + K8s CronJob
- LLM: OpenAI or Anthropic via env key; summarization only
- Auth: shared secret header on `/audit/trigger`; GitHub webhook HMAC when webhooks land

## Constraints

- **Tech stack**: Stay on Go + stdlib HTTP orchestrator; add drivers/SDKs only as needed (pgx, AWS SDK, Slack, GitHub API client, LLM SDK)
- ** determinism**: Metric collection remains deterministic code; LLM only for executive summary / narrative
- **Security**: Read-only GitHub/GitLab tokens; AWS ReadOnly + Cost Explorer; no secrets in repo
- **Performance**: Full audit target &lt;15 min for ~28 repos; FinOps-only &lt;3 min; incremental &lt;90s (from TDD)
- **Compatibility**: Keep demo sources and in-memory path for tests/local demos alongside live mode
- **Docs**: TDD.md remains design reference; README must describe what the binary actually does

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Full TDD parity as milestone goal | User chose option 3 over ops-only or live-only | — Pending |
| Phase order: ops → live → narrative | Ops unblocks safe live calls; live data unblocks trustworthy narrative | — Pending |
| Keep Go orchestrator (not LangGraph rewrite) | Working codebase; topology already matches TDD | — Pending |
| Auth via shared secret (+ webhook HMAC later) | Simple, portable; matches CronJob/API trigger model | — Pending |
| LLM summarization only | Matches TDD trade-off (cost + determinism) | — Pending |
| K8s CronJob + Docker deploy | Matches TDD operational considerations | — Pending |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd-transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd-complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-07-12 after initialization*
