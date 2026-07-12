# Feature Research

**Domain:** Platform maturity audit / FinOps / DORA engineering intelligence (Go audit agent → production TDD parity)
**Researched:** 2026-07-12
**Confidence:** HIGH (project scope from PROJECT.md + TDD.md + CONCERNS.md; ecosystem patterns from SEI/FinOps product landscape)

## Feature Landscape

### Table Stakes (Users Expect These)

Features operators and leadership assume exist once the agent is called “production-ready.” Missing these = demo, not a trusted audit platform. For **this milestone**, table stakes = full TDD parity foundations + live loop + honest scores (not a greenfield MVP subset).

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Authenticated audit trigger | Unauthenticated `POST /audit/trigger` is abuse + quota burn once live APIs exist | LOW | Shared-secret header (approved); GitHub webhook HMAC when webhooks land |
| Async audit jobs | Full audit can run ≤15 min; sync HTTP will time out at ingress/LB | MEDIUM | `202` + `audit_run_id`; `GET /audit/runs/{id}`; short HTTP timeouts |
| Durable Postgres persistence | Restart loses history; Grafana/dashboard path needs `audit_runs` / `repo_scores` / `findings` | MEDIUM | Schema exists in `migrations/001_schema.sql`; wire pgx `Store`; keep `Memory` for tests |
| Materialized health view refresh | Leadership “always-current” dashboard depends on `platform_health_current` | LOW–MEDIUM | `REFRESH MATERIALIZED VIEW` after successful write |
| Health / readiness endpoints | K8s CronJob + long-running API need probes | LOW | `/healthz` (liveness), `/readyz` (DB/deps) |
| Structured ops metrics + run ID logs | Ops cannot SLO or debug PARTIAL audits without them | MEDIUM | Per-node duration/error/findings counters; correlate on `audit_run_id` |
| Bounded concurrency + API rate limits | Unbounded fan-out → GitHub 429 / AWS throttle at ~28 repos | MEDIUM | Semaphore/`errgroup` (e.g. 8–16); client-side rate limiter |
| Live GitHub/GitLab repo + CI adapters | Demo fixtures ≠ org truth; standardization/CI scores must hit real trees/workflows | HIGH | Behind `demo\|live`; read-only tokens; pagination + retries |
| Live AWS FinOps inventory + Cost Explorer | FinOps value is estimated waste USD from real idle/over-provisioned/orphaned/untagged resources | HIGH | ReadOnly + Cost Explorer; CloudWatch utilization windows per TDD |
| Live Codecov / coverage fetch | Coverage risk scoring without live reports is fiction | MEDIUM | Use existing JaCoCo/JSON parsers end-to-end |
| Slack digest notifier | Scheduled audits without a human-facing digest stay invisible | MEDIUM | Webhook or Web API; digest failures non-fatal after DB success |
| Secrets via env / Secrets Manager pattern | Tokens in repo or ad-hoc env dumps fail security review | LOW–MEDIUM | Wire pattern now; K8s/Secrets Manager in deploy path |
| Honest DORA composites | Zeroed lead-time/CFR/MTTR currently score ~7.5 “healthy” — trust-destroying | MEDIUM | Require explicit metrics or “unknown”; never treat unset as perfect |
| Finding IDs + Postgres-ready rows | Schema requires UUID PK; empty IDs block persistence | LOW | Assign in workers or aggregator; map FinOps `service` tag → repo where possible |
| Non-zero priority for non-FinOps findings | All CI/coverage/standardization findings rank at priority 0 today | LOW–MEDIUM | Severity baseline cost or composite `(severity, type, effort)` |
| Branch protection + required reviewers | TDD repo-scanner governance signal; file-presence alone understates maturity | MEDIUM | GitHub branch protection API; extend standardization rubric |
| Audit scopes: full / incremental / finops_only | Operators expect targeted runs (weekly full, post-merge delta, cost-only) | LOW | Already planned in supervisor; keep + harden for live |
| Demo\|live dual mode | Local tests/CI must stay deterministic without credentials | LOW | Flag/env select; never drop demo path |
| ~28-repo scale path | Capacity claims and CronJob SLO assume estate size, not 5 fixture repos | MEDIUM | Expand manifest / load test; concurrency + rate limits prerequisite |
| Containerized K8s CronJob + on-demand HTTP | TDD operational model; weekly autonomy without manual `curl` | MEDIUM | Docker already present; add CronJob/manifests + secrets |
| Append-only findings + PARTIAL_AUDIT semantics | Production audits fail partially; must not invent completeness | MEDIUM | Transactional save preferred; notifier decoupled from DB success |

### Differentiators (Competitive Advantage)

Where this product wins vs buying LinearB/Jellyfish (SEI) *or* CloudHealth/Kubecost (FinOps) alone — and vs leaving the portfolio as a demo binary.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Unified CI/CD + coverage + FinOps + DORA in one scheduled agent | One ranked findings list instead of stitching SEI + FinOps dashboards | Already partially built | Defend by shipping live adapters + honest scores; do not split into three products |
| Cost × severity / effort ranking with actionable findings | Leadership wants “what to fix next,” not raw metric grids | MEDIUM | Fix priority-0 bug; keep ranking formula visible/deterministic |
| LLM executive summary (summarization only) | Narrative top-N digest for Slack/exec without LLM driving metrics | MEDIUM | OpenAI or Anthropic via env; ~50K tokens/run budget per TDD; never agentic collection |
| Deterministic metric collection + LLM narrative boundary | Auditable, repeatable scores; competitors often blur AI into scoring | LOW (policy) + MEDIUM (impl) | Core differentiator vs “AI platform auditor” hype |
| Incremental post-merge path (webhook → score delta) | Continuous signal without weekly-full cost; closes TDD Path 3 | HIGH | Needs auth HMAC, async job, delta vs last full run; PR comment optional P2 |
| Org template / standardization drift signals | Surfaces “not on golden path” beyond file presence | MEDIUM–HIGH | Branch protection first (table stakes); YAML template similarity is stretch |
| FinOps flags tied to ownership tags (`team`/`service`) | Waste attributed to owners, not anonymous resource IDs | MEDIUM | Required for FinOps actionability; enrich findings `owner_tag` / `RepoName` |
| Cron autonomy with zero manual audit effort | Replaces 2–3 engineer-weeks/quarter manual audits (TDD results claim) | MEDIUM | CronJob + Slack + Postgres history = closed loop |
| Portfolio-credible Go orchestrator at TDD topology | Same supervisor/worker shape without LangGraph rewrite | LOW | Docs alignment is part of the differentiator story |

### Anti-Features (Commonly Requested, Often Problematic)

Align with PROJECT.md **Out of Scope** and TDD non-goals. Deliberately do **not** build in this milestone.

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| Auto-remediate / open remediation PRs | “Close the loop” automation | Scope explosion; blast radius; TDD non-goal; humans act | Ranked findings + Slack; humans open PRs |
| Full agentic LLM metric collection | “Smarter” audits | Non-determinism, cost, unverifiable scores | Deterministic adapters; LLM summarization only |
| Replace APM (Datadog/Grafana) | One pane of glass | Wrong product boundary; runtime SLOs ≠ maturity rubrics | Emit metrics *into* Grafana over Postgres MV |
| Application-layer code quality audits | Broader “platform health” | Infinite scope (linters, complexity, smells) | Keep CI/coverage/standardization rubrics only |
| External vendor / SaaS dependency audits | Supply-chain buzz | Different data plane and compliance model | Out of scope; separate tool |
| Python/LangGraph rewrite | Match original TDD narrative literally | Throws away working Go orchestrator | Keep Go; docs: LangGraph-shaped topology appendix |
| Multi-cloud FinOps (Azure/GCP) or Kubecost-depth K8s costing | Feature parity with CloudHealth/Kubecost | Dilutes AWS-first TDD path; high complexity | AWS Cost Explorer + EC2/RDS/EBS flags only |
| DevEx surveys / SPACE / DX Core 4 | SEI market expectation | Qualitative product; different UX and cadence | Out of scope; DORA from delivery signals only |
| Investment allocation / R&D capitalization (Jellyfish-class) | Board finance narrative | Needs PM systems + time models | Stay on platform maturity + waste USD |
| Continuous polling of all APIs | “Real-time” health | Rate limits, cost, noise | Weekly cron + incremental webhook |
| Secret-scanning product (gitleaks-as-a-service) | Stronger secret hygiene | Heuristic rubric is enough for maturity score | Keep regex scoring; optionally note false-negatives |
| Multi-tenant SaaS / SSO / RBAC product shell | “Production platform” confusion | This is an internal agent, not a vendor product | Shared secret + network policy + CronJob |
| Self-service remediation marketplace / Backstage plugins | Platform catalog completeness | Future work in TDD open questions | Defer to later milestone |
| Cost forecasting / 90-day trajectories | FinOps maturity theater | Needs history + models; premature before live waste flags work | Persist weekly runs; forecast later |

## Feature Dependencies

```
Auth (shared secret)
    └──enables──> Safe live triggers / CronJob HTTP
Webhook HMAC
    └──requires──> Auth patterns + async jobs
    └──enables──> Incremental post-merge path

Postgres Store + migrations
    └──requires──> Finding IDs, transactional-ish write
    └──enables──> MV refresh, history, Grafana
    └──enables──> Async job status API

Async jobs (202 + poll)
    └──requires──> Durable run record (Postgres preferred)
    └──enables──> 28-repo full audits without HTTP timeout
    └──enables──> CronJob + on-demand coexistence

Bounded concurrency + rate limits
    └──requires──> Live adapters (or load tests against stubs)
    └──enables──> ~28-repo scale without PARTIAL from 429s

Live GitHub/GitLab
    └──requires──> Auth, secrets, rate limits, demo|live switch
    └──enables──> Branch protection / required reviewers
    └──enables──> Honest CI/standardization scores
    └──enhances──> Incremental webhook path

Live AWS FinOps
    └──requires──> Secrets/IAM, rate limits, Postgres for history
    └──enables──> Real waste USD + FinOps ranking signal

Live Codecov / coverage
    └──requires──> Live GitHub identity of repos; parsers wired
    └──enables──> Change-risk findings that leadership trusts

Slack digest
    └──requires──> Aggregated findings + preferably Postgres run
    └──enhances──> Cron autonomy (human-visible loop)
    └──conflicts──> Treating notifier failure as hard write failure
                    (decouple: persist first, notify best-effort)

Honest DORA + priority ranking fixes
    └──requires──> Real (or explicitly unknown) delivery metrics inputs
    └──requires──> Finding IDs / cost baselines for non-FinOps
    └──enables──> Leadership narrative that is not misleading
    └──enables──> LLM executive summary (summary of truth, not fiction)

LLM executive summary
    └──requires──> Aggregator output + honest ranking/DORA
    └──requires──> Secrets for LLM key
    └──conflicts──> Agentic LLM collection (anti-feature)

K8s CronJob deploy
    └──requires──> Container image, secrets pattern, healthz, auth
    └──enhances──> Weekly full + optional finops_only schedules

~28-repo scale
    └──requires──> Async jobs + concurrency caps + live/demo load path
    └──enhances──> Credibility of capacity claims
```

### Dependency Notes

- **Live adapters require trust & ops first:** Auth, async jobs, Postgres, and rate limits prevent unsafe/unobservable live runs (matches approved phase order: ops → live → narrative).
- **LLM summary requires honest ranking/DORA:** Summarizing placeholder “healthy” DORA or priority-0 CI findings amplifies false confidence.
- **Slack must not gate persistence:** Writer today fails the run if notifier errors; production must persist then notify.
- **Incremental webhook conflicts with sync HTTP:** Path 3 needs async + HMAC; do not bolt webhook onto blocking handler.
- **Branch protection requires live GitHub:** Cannot fake meaningfully in demo beyond fixtures; schedule after GitHub adapter.

## MVP Definition

For **this subsequent milestone**, “MVP” = minimum for **production TDD parity** (not the already-shipped demo). Align to agreed build order.

### Launch With (v1 — Trust & ops → Live loop → Leadership narrative)

**Track 1 — Trust & ops**
- [ ] Shared-secret auth on `/audit/trigger`
- [ ] Async jobs (`202` + run status API)
- [ ] Postgres store + migrations + MV refresh
- [ ] Health/readiness + structured metrics/`audit_run_id` logs
- [ ] Bounded worker concurrency + client rate limiting
- [ ] Finding IDs; transactional or all-or-nothing audit save; Slack errors non-fatal

**Track 2 — Live audit loop**
- [ ] `demo|live` source switch
- [ ] Live GitHub (and GitLab if in-scope for estate) adapters
- [ ] Live AWS FinOps (Cost Explorer + inventory/utilization flags)
- [ ] Live Codecov / coverage fetch through existing parsers
- [ ] Real Slack digests
- [ ] Secrets via env / Secrets Manager pattern

**Track 3 — Leadership narrative**
- [ ] Fix DORA unset→perfect bug; fix priority-0 non-FinOps ranking
- [ ] Branch protection + required reviewers in repo scanner
- [ ] LLM executive summary (summarization only)
- [ ] Scale path validated toward ~28 repos (manifest + load/smoke)
- [ ] K8s CronJob (+ on-demand HTTP) deploy docs/manifests
- [ ] Docs aligned to Go orchestrator (LangGraph as historical appendix)

### Add After Validation (v1.x)

- [ ] GitHub webhook incremental path + PR score-delta comment — after async + HMAC + live GitHub stable
- [ ] GraphQL batching / ETag caching for GitHub — if rate limits bite at 28 repos
- [ ] PagerDuty alert on audit SLO breach (>30 min) — after metrics exist
- [ ] Stronger secret-hygiene integration (gitleaks-style) — if regex false negatives dominate
- [ ] Ownership enrichment from service catalog (Backstage) — when tag coverage is insufficient

### Future Consideration (v2+)

- [ ] Auto-remediation PRs — explicitly out of scope now
- [ ] Kubernetes HPA / request-limit drift node — TDD open question
- [ ] Multi-cloud FinOps / FOCUS export — only if estate leaves AWS-only
- [ ] Cost forecasting (90-day) — needs multi-run history first
- [ ] Template cosine-similarity drift — after branch protection lands
- [ ] DevEx surveys / investment allocation — different product class

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| Auth (shared secret) | HIGH | LOW | P1 |
| Async audit jobs + status API | HIGH | MEDIUM | P1 |
| Postgres persistence + MV refresh | HIGH | MEDIUM | P1 |
| Healthz/readyz + run metrics/logs | HIGH | LOW–MEDIUM | P1 |
| Bounded concurrency + rate limits | HIGH | MEDIUM | P1 |
| Finding IDs + write/notify decoupling | HIGH | LOW | P1 |
| Live GitHub/GitLab adapters | HIGH | HIGH | P1 |
| Live AWS FinOps | HIGH | HIGH | P1 |
| Live Codecov/coverage | HIGH | MEDIUM | P1 |
| Slack digests (best-effort) | HIGH | MEDIUM | P1 |
| Secrets pattern (env/SM) | HIGH | LOW–MEDIUM | P1 |
| Honest DORA + priority ranking fixes | HIGH | MEDIUM | P1 |
| Branch protection / required reviewers | HIGH | MEDIUM | P1 |
| LLM executive summary (only) | MEDIUM–HIGH | MEDIUM | P1 |
| ~28-repo scale validation | HIGH | MEDIUM | P1 |
| K8s CronJob deploy | HIGH | MEDIUM | P1 |
| Docs ↔ Go reality alignment | MEDIUM | LOW | P1 |
| demo\|live dual mode | HIGH | LOW | P1 |
| Webhook incremental + PR comment | MEDIUM | HIGH | P2 |
| GitHub GraphQL/ETag optimization | MEDIUM | MEDIUM | P2 |
| PagerDuty SLO alerting | MEDIUM | LOW | P2 |
| Template YAML drift scoring | LOW–MEDIUM | HIGH | P3 |
| K8s resource audit node | MEDIUM | HIGH | P3 |
| Auto-remediate PRs | HIGH (requested) | HIGH | Anti / out of scope |
| Agentic LLM collection | MEDIUM (hype) | HIGH | Anti / out of scope |
| Multi-cloud / Kubecost-depth FinOps | MEDIUM | HIGH | Anti (this milestone) |
| APM replacement / app code quality | MEDIUM | HIGH | Anti / out of scope |

**Priority key:**
- P1: Must have for this milestone (full TDD parity)
- P2: Should have once P1 live loop is stable
- P3: Nice to have / later milestone
- Anti: Explicitly do not build

## Competitor Feature Analysis

| Feature | SEI (LinearB / Jellyfish / DX / Faros) | FinOps (CloudHealth / Kubecost / Vantage) | Our Approach (this agent) |
|---------|----------------------------------------|---------------------------------------------|---------------------------|
| DORA / delivery metrics | Table stakes; deep PR/cycle analytics | Rarely primary | Honest DORA composites from collected signals; no fake defaults |
| DevEx surveys / SPACE | DX differentiator | N/A | Anti-feature for this milestone |
| Investment / R&D capitalization | Jellyfish differentiator | N/A | Out of scope |
| CI/CD maturity rubrics | Partial (workflow automation in LinearB) | N/A | First-class 0–10 rubric + secret hygiene heuristic |
| Repo standardization / branch protection | Policy bots (partial) | N/A | Scanner + GitHub protection APIs |
| Coverage risk | Unusual as core | N/A | Codecov-backed change-risk findings |
| Cloud waste / rightsizing | Usually absent | Table stakes | AWS IDLE / OVER_PROVISIONED / ORPHANED / UNTAGGED |
| Ranked actionable findings | Insights vary; often dashboards | Recommendations + sometimes auto-actions | Deterministic cost×severity/effort ranking |
| Slack / scheduled digests | Common | Common | Table stakes notifier |
| Auto-remediation | LinearB gitStream (PR workflow) | Often rightsizing automation | Explicit non-goal — identify only |
| LLM narrative summary | Emerging “AI insights” | Emerging anomaly copy | Summarization only; metrics stay code |
| Auth / async / durable store | Assumed in SaaS | Assumed in SaaS | Must build (self-hosted Go agent) |
| Multi-cloud / K8s cost allocation | N/A | Table stakes for platforms | AWS-only; K8s deep cost deferred |

**Market takeaway:** SEI and FinOps vendors rarely ship *both* maturity rubrics and cloud-waste findings in one autonomous agent. The differentiator is the **unified ranked audit**; credibility depends on **ops table stakes + live data + non-lying DORA/priority** — not on matching Jellyfish finance features or CloudHealth multi-cloud breadth.

## Sources

- Project: `.planning/PROJECT.md`, `TDD.md`, `.planning/codebase/CONCERNS.md` (2026-07-12)
- CNCF TAG App Delivery — Platform Engineering Maturity Model (measurement / ops dimensions)
- DORA — Platform engineering capability guidance (delivery metrics as outcome signals)
- SEI landscape comparisons: LinearB, Jellyfish, DX, Faros, Swarmia (DORA table stakes; automation vs exec reporting vs DevEx surveys)
- FinOps landscape: AWS Cost Explorer baseline; CloudHealth / Kubecost / Vantage (visibility, tagging, rightsizing, anomaly — auto-remediation common but out of scope here)
- FinOps Foundation-aligned expectations: tagging governance, allocation, waste identification (unit economics / multi-cloud deferred)

---
*Feature research for: Platform Maturity Audit Agent — production TDD parity milestone*
*Researched: 2026-07-12*
