# Roadmap: Platform Maturity Audit Agent — Production Ready

## Overview

Brownfield path from demo/in-memory audit agent to full TDD parity: operators trigger authenticated async audits that persist to Postgres, pull live GitHub/GitLab + AWS + Codecov signals, notify Slack, and produce honest ranked findings with optional LLM narrative — then schedule the same path via K8s CronJob. Delivery follows **ops → live → narrative** across 12 fine phases under three tracks (Trust & ops, Live audit loop, Leadership narrative & deploy). Beads epic `build-agent-smith-1sf` tracks the same themes; each phase is sized to become a Beads child issue.

## Milestones

- 🚧 **v1 Production Ready** — Phases 1–12 (in progress)
- 📋 **v2 Incremental & Scale** — HOOK-01, GH-02, ALERT-01, SEC-01, OWN-01 (deferred)

## Phases

**Tracks:** 1–5 Trust & ops · 6–9 Live audit loop · 10–12 Leadership narrative & deploy

- [ ] **Phase 1: Finding Identity & Write Contract** - UUIDs, FinOps tag→repo, transactional persist, TDD/race bar, Beads tracking
- [ ] **Phase 2: Postgres Persistence** - Wire `store.Postgres`, migrations, MV refresh after successful write
- [ ] **Phase 3: Authenticated Trigger API** - Shared-secret auth, opaque client errors, safe body decode
- [ ] **Phase 4: Async Audit Jobs** - `202` + `audit_run_id`, status poll, detached job context
- [ ] **Phase 5: Concurrency Bounds & Observability** - Worker semaphore, PARTIAL_AUDIT attribution, health/metrics
- [ ] **Phase 6: Demo|Live Source Factory & Secrets** - Flag/env source switch; secrets from env/SM only
- [ ] **Phase 7: Live SCM Adapters & Rate Limits** - GitHub + GitLab clients with 429/5xx backoff
- [ ] **Phase 8: Live Coverage & AWS FinOps** - Codecov/artifacts + Cost Explorer/inventory waste findings
- [ ] **Phase 9: Slack Digests** - Real Slack notifier; failures logged and non-fatal
- [ ] **Phase 10: Honest Scoring & Branch Protection** - Unknown-safe DORA, non-FinOps priority, protection checks
- [ ] **Phase 11: LLM Executive Summary** - Post-aggregate summarization only; skip cleanly without key
- [ ] **Phase 12: Scale, Deploy & Docs** - ~28-repo path, K8s CronJob/Deployment, README + GoDoc parity

## Phase Details

### Phase 1: Finding Identity & Write Contract
**Goal**: Findings are insert-ready with stable IDs and a write path that never loses a successful save to notifier failure
**Depends on**: Nothing (first phase)
**Requirements**: PERS-02, PERS-03, QUAL-01, QUAL-03
**Success Criteria** (what must be TRUE):
  1. Every finding carries a UUID before any store insert; FinOps findings with a `service` tag map that tag to `repo_name`
  2. A successful persist commits even when the notifier fails; notifier failure does not mark a saved audit as rolled back
  3. Changed packages in this phase pass table-driven tests under `go test -race`
  4. Milestone work (bugs, features, blockers, handoffs) is tracked in Beads under epic `build-agent-smith-1sf`, not markdown TODOs
**Plans**: TBD

### Phase 2: Postgres Persistence
**Goal**: Audit history survives restarts via Postgres using the existing migration schema
**Depends on**: Phase 1
**Requirements**: PERS-01, PERS-04
**Success Criteria** (what must be TRUE):
  1. With `DATABASE_URL` set, audit runs, repo scores, and findings persist to Postgres via `migrations/`
  2. After a successful write, `platform_health_current` is refreshed and readable
  3. Memory store still works for tests/demo when Postgres is not configured
**Plans**: TBD

### Phase 3: Authenticated Trigger API
**Goal**: Only authenticated operators can trigger audits; clients never see internal error detail
**Depends on**: Phase 2
**Requirements**: AUTH-01, AUTH-02, AUTH-03
**Success Criteria** (what must be TRUE):
  1. `POST /audit/trigger` without a valid shared-secret header is rejected
  2. API responses to clients stay opaque on failure while full errors appear only in server logs
  3. Oversized or empty bodies are handled safely (MaxBytesReader; empty body defaults to full scope) without panics
**Plans**: TBD

### Phase 4: Async Audit Jobs
**Goal**: Trigger returns immediately; operators poll durable run status until the audit finishes
**Depends on**: Phase 3
**Requirements**: JOBS-01, JOBS-02, JOBS-03
**Success Criteria** (what must be TRUE):
  1. `POST /audit/trigger` returns `202` with `audit_run_id` without waiting for audit completion
  2. Operator can poll `GET /audit/runs/{id}` for `PENDING|RUNNING|COMPLETE|PARTIAL_AUDIT|FAILED` and finding counts
  3. Background audit jobs continue after the HTTP request ends (request context cancellation does not abort the run)
  4. Job lifecycle status transitions are covered by table-driven tests with `go test -race`
**Plans**: TBD

### Phase 5: Concurrency Bounds & Observability
**Goal**: Fan-out is bounded and operators can see process health, readiness, and per-run telemetry
**Depends on**: Phase 4
**Requirements**: CONC-01, CONC-03, OBS-01, OBS-02
**Success Criteria** (what must be TRUE):
  1. Worker fan-out respects a configurable semaphore/errgroup limit (default 8–16)
  2. When any worker fails, run status is `PARTIAL_AUDIT` and errors attribute node and repo
  3. `/healthz` and `/readyz` report process and dependency readiness (including Postgres when configured)
  4. Structured logs/metrics include `audit_run_id`, node durations, error counts, and findings produced
**Plans**: TBD

### Phase 6: Demo|Live Source Factory & Secrets
**Goal**: Operators switch demo vs live sources without code changes; credentials never land in repo or logs
**Depends on**: Phase 5
**Requirements**: LIVE-01, LIVE-06
**Success Criteria** (what must be TRUE):
  1. Flag/env selects `demo|live` sources and wires `agent.Dependencies` without editing worker code
  2. Secrets load from env (and optional Secrets Manager); CI/demo paths run with no live credentials
  3. Credential values never appear in structured logs under test or demo runs
**Plans**: TBD

### Phase 7: Live SCM Adapters & Rate Limits
**Goal**: Live GitHub and GitLab supply repo/CI metadata behind rate-limited clients
**Depends on**: Phase 6
**Requirements**: LIVE-02, LIVE-03, CONC-02
**Success Criteria** (what must be TRUE):
  1. Live GitHub adapter fetches repo files, workflow YAML, and related metadata via `agent.Dependencies`
  2. Live GitLab adapter covers manifest entries with `provider: gitlab`
  3. Live API clients apply rate limiting and backoff on 429/5xx without unbounded retry storms
  4. Live adapters are covered by fakes/stubs so package tests pass under `go test -race` without network credentials
**Plans**: TBD

### Phase 8: Live Coverage & AWS FinOps
**Goal**: Live coverage and AWS inventory produce real coverage scores and waste findings
**Depends on**: Phase 7
**Requirements**: LIVE-04, LIVE-05
**Success Criteria** (what must be TRUE):
  1. Live AWS FinOps adapter emits IDLE / OVER_PROVISIONED / ORPHANED / UNTAGGED findings from Cost Explorer + inventory/utilization
  2. Live coverage fetch uses Codecov and/or CI artifacts through existing JaCoCo/JSON parsers
  3. Demo mode still produces equivalent finding shapes for local/CI without AWS or Codecov credentials
**Plans**: TBD

### Phase 9: Slack Digests
**Goal**: Successful audits notify Slack with top findings; Slack outages never fail the audit
**Depends on**: Phase 8
**Requirements**: SLACK-01, SLACK-02
**Success Criteria** (what must be TRUE):
  1. Successful audits emit a real Slack digest (webhook or bot) with top findings and waste totals
  2. Slack failures are logged and leave the audit run status based on persist/worker outcome, not notifier errors
**Plans**: TBD

### Phase 10: Honest Scoring & Branch Protection
**Goal**: Leadership digests show trustworthy DORA/priority signals and real branch-protection posture
**Depends on**: Phase 9
**Requirements**: DORA-01, RANK-01, SCAN-01
**Success Criteria** (what must be TRUE):
  1. Unset/unknown delivery metrics do not score as perfect; digests omit DORA composite until observations exist
  2. Non-FinOps findings receive non-zero priority via severity/effort-aware ranking (not cost-only zeros)
  3. Repo scanner evaluates branch protection and required reviewers, not only file presence
  4. Scoring/ranking behavior is locked by table-driven tests under `go test -race`
**Plans**: TBD

### Phase 11: LLM Executive Summary
**Goal**: Optional post-aggregate LLM narrative summarizes ranked findings without inventing metrics
**Depends on**: Phase 10
**Requirements**: LLM-01, LLM-02
**Success Criteria** (what must be TRUE):
  1. After aggregation, an optional LLM produces an executive summary from ranked findings only (no metric invention)
  2. When no API key is configured, the LLM step is skipped cleanly and findings/persist/notify still complete
**Plans**: TBD

### Phase 12: Scale, Deploy & Docs
**Goal**: The authenticated audit path runs on a ~28-repo estate via K8s and is documented as the Go orchestrator it is
**Depends on**: Phase 11
**Requirements**: SCALE-01, DEPL-01, DOCS-01, QUAL-02
**Success Criteria** (what must be TRUE):
  1. Manifest and smoke/load path validate toward ~28 repos without unbounded concurrency collapse
  2. Docker image + K8s CronJob (and on-demand Deployment) can schedule authenticated full audits
  3. README describes the Go orchestrator accurately; LangGraph claims are historical/appendix only
  4. Exported packages/types/functions have GoDoc suitable for operators and contributors
**Plans**: TBD

## Progress

**Execution Order:**
1 → 2 → 3 → 4 → 5 → 6 → 7 → 8 → 9 → 10 → 11 → 12

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Finding Identity & Write Contract | 0/TBD | Not started | - |
| 2. Postgres Persistence | 0/TBD | Not started | - |
| 3. Authenticated Trigger API | 0/TBD | Not started | - |
| 4. Async Audit Jobs | 0/TBD | Not started | - |
| 5. Concurrency Bounds & Observability | 0/TBD | Not started | - |
| 6. Demo\|Live Source Factory & Secrets | 0/TBD | Not started | - |
| 7. Live SCM Adapters & Rate Limits | 0/TBD | Not started | - |
| 8. Live Coverage & AWS FinOps | 0/TBD | Not started | - |
| 9. Slack Digests | 0/TBD | Not started | - |
| 10. Honest Scoring & Branch Protection | 0/TBD | Not started | - |
| 11. LLM Executive Summary | 0/TBD | Not started | - |
| 12. Scale, Deploy & Docs | 0/TBD | Not started | - |

## Coverage Summary

| Track | Phases | Requirement count |
|-------|--------|-------------------|
| Trust & ops | 1–5 | 16 |
| Live audit loop | 6–9 | 11 |
| Leadership narrative & deploy | 10–12 | 7 |
| **Total** | **12** | **34/34** |

v2 deferred (not mapped): HOOK-01, GH-02, ALERT-01, SEC-01, OWN-01
---
*Roadmap created: 2026-07-12*
*Granularity: fine (12 phases)*
*Beads epic: build-agent-smith-1sf*
