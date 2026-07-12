# Requirements: Platform Maturity Audit Agent — Production Ready

**Defined:** 2026-07-12  
**Core Value:** Engineering leadership can trust a scheduled audit run to produce accurate, persisted, ranked findings from live GitHub/GitLab + AWS data — without manual audit effort.

## v1 Requirements

Requirements for the production-ready / full TDD parity milestone. Each maps to roadmap phases.

### Ops Security & API

- [ ] **AUTH-01**: Operator can trigger audits only with a valid shared-secret header on `POST /audit/trigger`
- [ ] **AUTH-02**: API returns opaque errors to clients; full errors stay in server logs
- [ ] **AUTH-03**: Trigger request body is always decoded safely (MaxBytesReader; empty body defaults to full scope)
- [ ] **JOBS-01**: `POST /audit/trigger` returns `202` with `audit_run_id` without waiting for audit completion
- [ ] **JOBS-02**: Operator can poll `GET /audit/runs/{id}` for status (`PENDING|RUNNING|COMPLETE|PARTIAL_AUDIT|FAILED`) and finding counts
- [ ] **JOBS-03**: Background audit jobs do not cancel when the HTTP request context ends

### Persistence & Observability

- [ ] **PERS-01**: Audit runs, repo scores, and findings persist to Postgres via migrations in `migrations/`
- [ ] **PERS-02**: Every finding has a UUID primary key before insert; FinOps findings map `service` tag to `repo_name` when present
- [ ] **PERS-03**: Persist path is transactional (or all-or-nothing); Slack/notifier failure does not roll back a successful save
- [ ] **PERS-04**: `platform_health_current` materialized view refreshes after a successful write
- [ ] **OBS-01**: `/healthz` and `/readyz` endpoints report process and dependency readiness (e.g. Postgres)
- [ ] **OBS-02**: Structured logs and metrics include `audit_run_id`, node durations, error counts, findings produced

### Concurrency & Reliability

- [ ] **CONC-01**: Worker fan-out is bounded (configurable semaphore / errgroup limit, default 8–16)
- [ ] **CONC-02**: Live API clients apply rate limiting and backoff on 429/5xx
- [ ] **CONC-03**: `PARTIAL_AUDIT` status is set when any worker fails; errors include node and repo attribution

### Live Data Sources

- [ ] **LIVE-01**: Operator can select `demo|live` sources via flag/env without code changes
- [ ] **LIVE-02**: Live GitHub adapter fetches repo files, workflow YAML, and related metadata behind `agent.Dependencies`
- [ ] **LIVE-03**: Live GitLab adapter covers estates that list `provider: gitlab` in the manifest
- [ ] **LIVE-04**: Live AWS FinOps adapter produces IDLE / OVER_PROVISIONED / ORPHANED / UNTAGGED findings from Cost Explorer + inventory/utilization
- [ ] **LIVE-05**: Live coverage fetch uses Codecov and/or CI artifacts through existing JaCoCo/JSON parsers
- [ ] **LIVE-06**: Secrets load from env (and optional Secrets Manager) — no credentials in repo or logs

### Notifications

- [ ] **SLACK-01**: Successful audits emit a real Slack digest (webhook or bot) with top findings and waste totals
- [ ] **SLACK-02**: Slack failures are logged and non-fatal to the audit run

### Scoring & Narrative

- [ ] **DORA-01**: Unset/unknown delivery metrics do not score as perfect; digests omit composite until observations exist
- [ ] **RANK-01**: Non-FinOps findings receive non-zero priority (severity/effort-aware ranking, not cost-only zeros)
- [ ] **SCAN-01**: Repo scanner evaluates branch protection and required reviewers (not only file presence)
- [ ] **LLM-01**: After aggregation, an optional LLM produces an executive summary from ranked findings only (summarization; no metric invention)
- [ ] **LLM-02**: LLM is skipped cleanly when no API key is configured

### Scale & Deploy

- [ ] **SCALE-01**: Manifest and smoke/load path validate toward ~28 repos without unbounded concurrency collapse
- [ ] **DEPL-01**: Docker image + K8s CronJob (and on-demand Deployment) can schedule authenticated full audits
- [ ] **DOCS-01**: README describes the Go orchestrator accurately; LangGraph claims are historical/appendix only

### Quality Bar

- [ ] **QUAL-01**: Changed packages keep table-driven tests with `go test -race`; live adapters covered by fakes/stubs
- [ ] **QUAL-02**: Exported packages/types/functions have GoDoc suitable for operators and contributors
- [ ] **QUAL-03**: Beads tracks bugs, features, blockers, and handoffs for this milestone (no markdown TODO SoT)

## v2 Requirements

Deferred after production parity validates.

### Incremental & Scale

- **HOOK-01**: GitHub webhook incremental audit + PR score-delta comment (after HMAC + live GitHub stable)
- **GH-02**: GraphQL batching / ETag caching when REST secondary rate limits bite at 28 repos
- **ALERT-01**: PagerDuty (or equivalent) when audit exceeds 30-minute SLO
- **SEC-01**: Stronger secret hygiene (gitleaks-style) beyond regex heuristics
- **OWN-01**: Ownership enrichment from Backstage / service catalog

## Out of Scope

| Feature | Reason |
|---------|--------|
| Auto-remediate / open fix PRs | TDD non-goal; humans act |
| Agentic LLM driving metric collection | Determinism + cost; summarization only |
| Replace APM (Datadog/Grafana) | Separate runtime concern |
| App-layer code quality audits | Separate concern |
| External vendor/SaaS dependency audits | TDD non-goal |
| Rewrite in Python/LangGraph | Keep Go supervisor/worker architecture |
| Multi-cloud FinOps / K8s deep cost node | AWS-only for this milestone |
| DevEx surveys / SPACE product | Different product class |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| AUTH-01 | Phase 3 | Pending |
| AUTH-02 | Phase 3 | Pending |
| AUTH-03 | Phase 3 | Pending |
| JOBS-01 | Phase 4 | Pending |
| JOBS-02 | Phase 4 | Pending |
| JOBS-03 | Phase 4 | Pending |
| PERS-01 | Phase 2 | Pending |
| PERS-02 | Phase 1 | Pending |
| PERS-03 | Phase 1 | Pending |
| PERS-04 | Phase 2 | Pending |
| OBS-01 | Phase 5 | Pending |
| OBS-02 | Phase 5 | Pending |
| CONC-01 | Phase 5 | Pending |
| CONC-02 | Phase 7 | Pending |
| CONC-03 | Phase 5 | Pending |
| LIVE-01 | Phase 6 | Pending |
| LIVE-02 | Phase 7 | Pending |
| LIVE-03 | Phase 7 | Pending |
| LIVE-04 | Phase 8 | Pending |
| LIVE-05 | Phase 8 | Pending |
| LIVE-06 | Phase 6 | Pending |
| SLACK-01 | Phase 9 | Pending |
| SLACK-02 | Phase 9 | Pending |
| DORA-01 | Phase 10 | Pending |
| RANK-01 | Phase 10 | Pending |
| SCAN-01 | Phase 10 | Pending |
| LLM-01 | Phase 11 | Pending |
| LLM-02 | Phase 11 | Pending |
| SCALE-01 | Phase 12 | Pending |
| DEPL-01 | Phase 12 | Pending |
| DOCS-01 | Phase 12 | Pending |
| QUAL-01 | Phase 1 | Pending |
| QUAL-02 | Phase 12 | Pending |
| QUAL-03 | Phase 1 | Pending |

**Coverage:**
- v1 requirements: 34 total
- Mapped to phases: 34
- Unmapped: 0 ✓

---
*Requirements defined: 2026-07-12*  
*Last updated: 2026-07-12 after roadmap creation*
