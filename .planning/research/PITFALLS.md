# Pitfalls Research

**Domain:** Platform maturity / FinOps audit agents (Go supervisor → live GitHub/AWS → Postgres → narrative)
**Researched:** 2026-07-12
**Confidence:** HIGH (codebase concerns + official GitHub/AWS docs + DORA implementation literature + Go testing practices)

## Critical Pitfalls

### Pitfall 1: Fake-Healthy Scores from Zero / Unset Metrics

**What goes wrong:**
DORA composites and maturity scores look “elite” when delivery metrics were never collected. Zero-valued `LeadTimeDays`, `ChangeFailureRate`, and `MTTRHours` map to perfect dimension scores; only `DeploysPerWeek=0` correctly scores low — composite ≈ 7.5. Leadership digests ship green numbers that mean “unknown,” not “healthy.”

**Why it happens:**
Demo fixtures and placeholder aggregators treat Go zero values as real observations. Teams rush narrative/dashboard phases before wiring deploy/incident telemetry. Industry DORA implementations compound this with inconsistent definitions (merge ≠ deploy; Sev-1-only CFR → near-zero failures).

**How to avoid:**
- Use explicit sentinels (`unknown` / nullable) — never score unset dimensions.
- Omit DORA from digests/API until at least N observations exist per dimension.
- Document org-wide definitions (what counts as deploy, failure, restore start/end) before computing composites.
- Assert in agent/e2e tests: unset metrics must not yield high composites.

**Warning signs:**
- Every repo’s `DoraCompositeScore` clusters near the same mid/high value.
- Digests claim “healthy DORA” while no deploy/CFR source is wired.
- Aggregator unit tests only feed hand-crafted non-zero metrics (happy path).

**Phase to address:**
Phase 3 (Leadership narrative) — fix before any LLM or Slack narrative references DORA; add guardrail tests in Phase 1 if scores are persisted early.

---

### Pitfall 2: Unbounded Concurrent Live API Fan-Out → 429 / Partial Audits

**What goes wrong:**
Per-repo goroutine fan-out (scanner + CI/CD + coverage × N repos + FinOps) hammers GitHub/AWS. Secondary rate limits fire; workers error; run status becomes `PARTIAL_AUDIT` with incomplete findings. Continuing to retry while limited can get the integration banned.

**Why it happens:**
Demo path has no network cost, so unbounded `WaitGroup` fan-out “works.” TDD capacity planning cites primary limits (~5k GitHub req/hr) and underweights secondary limits (concurrent request caps, points/minute). GitHub docs explicitly: make requests **serially** for a single user/client ID; avoid concurrent requests.

**How to avoid:**
- Bound concurrency (semaphore / `errgroup` + limit, e.g. 4–8).
- Shared rate-limit RoundTripper (respect `Retry-After`, `x-ratelimit-remaining` / `x-ratelimit-reset`).
- Prefer GraphQL batching + conditional requests (ETag / `If-None-Match`) so 304s don’t burn primary quota.
- Cache tree/contents per repo; one contents/tree call beats nested `HasFile` path probes.
- Fail closed after bounded retries — mark dimension failed, don’t hammer.

**Warning signs:**
- Spike in `PARTIAL_AUDIT` once `sources=live`.
- Logs full of 403/429 without backoff.
- Audit runtime grows non-linearly with repo count (retry storms).

**Phase to address:**
Phase 1 (Trust & ops) — concurrency + rate-limit skeleton; Phase 2 (Live audit loop) — wire into real clients.

---

### Pitfall 3: Synchronous HTTP Trigger = Full Audit Duration

**What goes wrong:**
`POST /audit/trigger` blocks until workers + write complete. At TDD targets (&lt;15 min / 28 repos), load balancers, ingress, and clients time out; connection pools exhaust; operators retry and spawn duplicate audits.

**Why it happens:**
Demo estate finishes sub-second, so sync handler looks fine. Teams add live clients without changing the request lifecycle.

**How to avoid:**
- Accept → validate → enqueue → `202 Accepted` + `audit_run_id` + `Location` / poll URL.
- Background runner with single-flight / lease per scope; CronJob `concurrencyPolicy: Forbid`.
- Short HTTP timeouts on the API server; long work only in job context.
- Status machine: `pending` → `running` → `completed` | `failed` | `partial`.

**Warning signs:**
- Smoke scripts need multi-minute curls; ingress 504s on full scope.
- Duplicate runs after client retries.
- No `GET /audit/runs/{id}`.

**Phase to address:**
Phase 1 (Trust & ops) — async jobs are a hard prerequisite before live sources.

---

### Pitfall 4: Unauthenticated / Over-Privileged Trigger + Live Credentials

**What goes wrong:**
Anyone who can reach the port starts full audits, burns GitHub/AWS quota, and exfiltrates findings. Over-scoped tokens (write access, broad IAM) turn a DoS into a blast radius. Error bodies leak upstream paths and resource IDs to clients.

**Why it happens:**
Local demo binds `:8080` with no auth. Live tokens get copied from personal PATs “just to make it work.” Shared-secret auth is deferred as “easy later.”

**How to avoid:**
- Shared-secret header on `/audit/trigger` before any live mode; webhook HMAC when webhooks land.
- Read-only GitHub/GitLab scopes; AWS `ReadOnlyAccess` + Cost Explorer only; prefer IRSA / instance role over long-lived keys.
- Secrets via env / Secrets Manager — never commit; rotate; separate demo vs prod credentials.
- Log full errors server-side; return opaque client errors.
- Rate-limit triggers; bind privately / require ingress auth.

**Warning signs:**
- Trigger works with no header in staging.
- PAT has `repo` write or org admin.
- 500 responses include AWS/GitHub error strings.

**Phase to address:**
Phase 1 (Trust & ops) — auth + error hygiene before Phase 2 live credentials.

---

### Pitfall 5: Non-Transactional Multi-Save + Notifier Coupled to Persistence

**What goes wrong:**
`SaveAuditRun` → `SaveRepoScores` → `SaveFindings` → notifier; mid-failure leaves orphan runs, missing findings, or “failed” API responses after data is already committed. Empty `Finding.ID` passes in-memory but violates Postgres UUID PK. Slack flakiness fails the whole write path.

**Why it happens:**
In-memory store is forgiving. Schema exists but wasn’t exercised. Notifier is treated as part of the critical path because the Writer interface returns one error.

**How to avoid:**
- Single transactional `Store.SaveAudit(state)` (or explicit compensate).
- Assign UUIDs in workers or aggregator before persist; map FinOps `service` tag → `RepoName`.
- Decouple notifier: persist success ≠ digest success (log + retry digest separately).
- Migration + store tests with testcontainers / embedded Postgres before cutover.
- Refresh materialized views explicitly after commit; document staleness.

**Warning signs:**
- `audit_runs` rows without findings; MV never updates.
- First Postgres enablement fails on empty IDs.
- Slack outage marks audit failed despite DB success.

**Phase to address:**
Phase 1 (Trust & ops) — Postgres store + IDs + transaction semantics.

---

### Pitfall 6: FinOps Waste Dollars That Don’t Survive Owner Review

**What goes wrong:**
Average / low-percentile CPU flags bursty API and JVM workloads as IDLE / OVER_PROVISIONED. Untagged resources can’t be attributed; digests claim large monthly waste that FinOps cannot act on. Cost Explorer alone is billed as “utilization audit” without CloudWatch / Compute Optimizer nuance.

**Why it happens:**
Simple thresholds (CPU &lt;5% over 14d) copy demo heuristics. Teams skip tag completeness and owner validation. Savings use On-Demand list price, not actual discounting.

**How to avoid:**
- Require lookback + peak/network signals (and/or Compute Optimizer idle APIs); treat average-only as advisory, not actionable.
- Gate “estimated waste” behind tag completeness (`team`/`service`); UNTAGGED before dollar claims.
- Separate finding severity: confidence tier (high = orphaned EBS; low = avg-CPU rightsizing).
- Route findings to owners; never auto-remediate from agent output.
- Document that Cost Explorer is spend; utilization needs CloudWatch/Optimizer.

**Warning signs:**
- Large `$` totals with mostly UNTAGGED / low-confidence flags.
- Engineers dispute every IDLE finding after first digest.
- No network/peak metrics in FinOps payloads.

**Phase to address:**
Phase 2 (Live audit loop) for signals/tags; Phase 3 for narrative confidence language.

---

### Pitfall 7: LLM Narrative Invents or Reorders Facts

**What goes wrong:**
Executive summary invents repos, dollar amounts, or “critical” items not in the ranked findings. Priority explanation diverges from deterministic `PriorityScore`. Leadership trusts prose over tables; bad decisions follow.

**Why it happens:**
LLM is asked to “summarize and prioritize” without hard grounding. Temperature &gt;0; no citation of finding IDs; no post-check that claimed numbers ⊆ structured payload.

**How to avoid:**
- Deterministic collection + ranking only; LLM summarization of **already ranked** top-N JSON.
- Prompt: numbers/repos only from input; refuse if missing; low temperature.
- Post-validate: every claimed `$` / repo / severity must match input finding IDs.
- Keep deterministic digest as fallback when LLM fails or fails validation.
- Never let LLM choose worker routing or mutate scores.

**Warning signs:**
- Summary mentions repos absent from `state.Findings`.
- Dollar totals in prose ≠ sum of ranked findings.
- No golden-fixture tests for summarizer grounding.

**Phase to address:**
Phase 3 (Leadership narrative) — after live data and fixed ranking exist.

---

### Pitfall 8: Priority Ranking That Buries Non-FinOps Work

**What goes wrong:**
`priority = cost × severity / effort` with `EstimatedCostUSD=0` for CI/CD, coverage, and standardization → all priority 0. Digests are FinOps-only; platform gaps never surface.

**Why it happens:**
Cost-centric formula copied from FinOps; other finders never set baseline impact weights.

**How to avoid:**
- Composite rank key: severity tier + type weight + optional cost; or severity-weighted baseline cost for non-cloud findings.
- Separate digest sections: “Cloud waste” vs “Platform gaps.”
- Tests with mixed zero-cost / positive-cost findings asserting interleaving rules.

**Warning signs:**
- All non-cloud findings share `PriorityScore == 0`.
- Top-10 digest is 100% cloud_waste after full audit.

**Phase to address:**
Phase 3 (Leadership narrative); schema/store must already persist priority fields (Phase 1).

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| `demo.Sources` only in main | Fast demos, no creds | False product claims; no live confidence | Local/tests forever; never as sole prod path |
| In-memory store | No DB ops | Lost history; no Grafana; unbounded RAM | Unit tests / ephemeral demos only |
| Sync HTTP audit | Simple handler | Timeouts, duplicate runs at real scale | Demo estate &lt; few seconds only |
| Unbounded goroutine fan-out | Simple orchestration | 429 storms, partial audits | Never once live APIs exist |
| Zero→perfect DORA mapping | Scores always present | Leadership misled | Never — omit or mark unknown |
| Log-only Slack notifier | No Slack secrets | Silent “success”; operators scrape logs | Dev only; prod needs real notifier + non-fatal failures |
| Heuristic secret regex as “hygiene score” | Cheap signal | False confidence vs gitleaks/trufflehog | Demo scoring only; don’t claim security audit |
| Docs claim LangGraph/28 repos while Go/5 repos ship | Narrative polish | Onboarding and capacity lies | Never — align docs in Phase 1 |

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| GitHub REST | Concurrent per-repo calls; ignore secondary limits | Serial/queued client; shared limiter; ETags; GraphQL batch |
| GitHub pagination | Hand-rolled URL prediction | Follow `Link` headers; don’t parse/construct page URLs |
| GitHub webhooks | Poll instead of subscribe; skip HMAC | Webhooks for incremental; verify signature before work |
| AWS Cost Explorer | Treat as utilization source; ignore monthly quotas | Use for spend; CloudWatch/Optimizer for idle/rightsizing; cache CE calls |
| CloudWatch / idle heuristics | Avg CPU only → false IDLE | Peak + network; longer lookback; confidence tiers |
| Codecov / coverage artifacts | Pass pre-parsed floats; skip parsers | Fetch raw XML/JSON → existing parsers → Analyze |
| Postgres | Insert without UUIDs; non-transactional multi-table | UUID at create; one transaction; test migrations |
| Slack | Fail write path on digest error; log secrets in messages | Async/non-fatal digest; never echo tokens/findings with secrets |
| LLM API | Agentic control of metrics; ungrounded prose | Summarize ranked JSON only; validate claims |
| Auth / secrets | Long-lived wide PATs in env files | Read-only scopes; Secrets Manager / IRSA; shared-secret trigger |

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Unbounded worker fan-out | 429s, PARTIAL_AUDIT | Concurrency cap + rate limiter | ~10+ repos live; worse at 28 |
| Nested `HasFile` path probes | Many round-trips/repo | Single tree/contents fetch | Full audit approaches 15-min SLO |
| Sync trigger | LB 504, retries | 202 + poll / CronJob job | Audits &gt; ~30–60s |
| Unbounded Memory store | Pod OOM over weeks | Postgres + retention | Long-lived process, weekly runs |
| Cost Explorer chatty loops | Throttle / month quota burn | Aggregate queries; cache 24h | Aggressive finops_only polling |
| No Write/Read timeouts | Hung connections | Set server timeouts; async work | Concurrent trigger abuse |
| CronJob `Allow` concurrency | Overlapping full audits | `Forbid` + lease | Audit runtime &gt; schedule interval |

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| Open `/audit/trigger` | Quota burn, finding exfil, DoS | Shared secret (+ mTLS/ingress); rate limit |
| Write-capable SCM/cloud creds | Accidental mutation / blast radius | Read-only tokens/roles; deny writes in IAM |
| Returning `err.Error()` to clients | Path/account leakage | Opaque 5xx; structured server logs |
| Logging secret-regex matches | Credential leakage in logs | Log finding type only; never matched substring |
| Cleartext HTTP beyond localhost | Findings/tokens sniffable | TLS at ingress; timeouts |
| Demo mode accidentally in prod | Fake audits trusted as real | Explicit `SOURCES=live` gate + startup banner/metric |
| Slack digest with resource ARNs to public channel | Internal topology exposure | Private channel; severity-gated content |

## UX Pitfalls (Operator / Leadership Consumers)

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| Green DORA without data | False confidence | Show “insufficient data”; hide composites |
| Top-10 all FinOps $ | Platform work invisible | Dual sections; non-zero priority for gaps |
| PARTIAL_AUDIT with no per-repo errors | Can’t triage | Set `AuditError.Repo`; surface failed dimensions |
| Sync trigger “hangs” | Operators kill/retry | 202 + status page / Slack “started” |
| Waste $ without owners | FinOps can’t assign | Require tags; UNTAGGED queue first |
| LLM prose contradicts table | Trust collapse | Grounded summary + link finding IDs |
| Docs say 28 repos / LangGraph | Wrong ops expectations | Align README to Go binary + manifest size |

## Testing Pitfalls (TDD Mode)

### Race Conditions in Orchestrator Tests

**What goes wrong:** Shared `findings`/`metrics`/`errs` under one mutex look fine until `-race` or cancellation tests run; context checked only after `wg.Wait` hides shutdown bugs.

**Prevention:**
- Always `go test -race ./...` in CI (project quality bar).
- Table-driven partial-failure + cancel-mid-run cases.
- Prefer `errgroup` with bounded parallelism and per-task ctx.

**Warning signs:** Flakes only under `-race` or high `-count`; missing cancel tests.

**Phase:** Phase 1 when touching agent coordination.

### Mocking Live APIs Incorrectly

**What goes wrong:** Mutating `http.DefaultTransport` / global httpmock → cross-test pollution and races. Happy-path stubs hide pagination, 429, and auth failures. First live cutover fails in production shapes.

**Prevention:**
- Inject `*http.Client` / base URL; use `httptest.Server` per test (no global transport swap).
- Contract tests: 401, 403, 404, 429 + `Retry-After`, pagination `Link`, empty pages.
- Keep `demo.Sources` for e2e determinism; never call real GitHub/AWS in unit CI.
- Optional recorded fixtures (VCR) for adapter golden paths — refresh deliberately.

**Warning signs:** Tests pass only with network; parallel `t.Parallel()` flakes; no 429 test.

**Phase:** Phase 2 with each live adapter.

### Postgres / Async Job Test Gaps

**What goes wrong:** Memory store tests green; Postgres rejects empty UUIDs; async handler tests never assert 202 vs blocking.

**Prevention:**
- testcontainers (or equivalent) for migrations + transactional save.
- Handler tests: auth failure, 202 + run id, duplicate trigger single-flight.
- Writer tests: notifier failure does not roll back successful persist (once decoupled).

**Phase:** Phase 1.

### Scoring / Ranking Test Blind Spots

**What goes wrong:** Aggregator tests only feed complete metrics; agent path never asserts placeholder DORA or zero-cost priority.

**Prevention:** Agent-level assertions on unknown metrics and mixed finding types; golden digests.

**Phase:** Phase 3 (with early guardrails if scores persist in Phase 1).

## "Looks Done But Isn't" Checklist

- [ ] **Auth on trigger:** Request without secret is 401 — verify middleware test + staging probe
- [ ] **Async jobs:** Full audit returns 202 quickly; status endpoint reaches terminal state — verify with live-sized fixture timing
- [ ] **Bounded concurrency + rate limit:** Live client respects limiter under parallel workers — verify with httptest 429 sequence
- [ ] **Postgres path:** Migrations apply; findings have UUIDs; transactional save; MV refresh — verify integration test
- [ ] **Notifier non-fatal:** Slack down still persists audit — verify writer test
- [ ] **DORA unknown handling:** Unset metrics do not yield ~7.5 composites — verify agent assertion
- [ ] **Priority for non-FinOps:** CI/coverage/standardization appear in ranked output — verify mixed fixture
- [ ] **FinOps confidence:** Waste $ not claimed for untagged/low-signal resources — verify analyzer rules
- [ ] **LLM grounding:** Summary claims ⊆ input finding IDs/amounts — verify validator tests
- [ ] **Demo/live switch:** Prod cannot silently run demo sources — verify startup config fail-closed
- [ ] **Health/metrics:** `/healthz` + run metrics exist before K8s probes depend on them
- [ ] **Docs parity:** README describes Go orchestrator, manifest size, and real capabilities

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Rate-limit ban / storm | MEDIUM | Stop triggers; reduce concurrency; wait reset; add limiter; re-run with `finops_only` or smaller manifest |
| Misleading DORA shipped | HIGH (trust) | Yank composites from digests; annotate historical rows `untrusted`; republish with unknown semantics |
| Partial Postgres writes | MEDIUM | Mark run `failed`/`partial`; delete or quarantine incomplete run; fix transaction; replay |
| Inflated FinOps $ | HIGH (trust) | Relabel findings low-confidence; require owner ack; tighten heuristics; issue correction digest |
| LLM hallucinated summary | LOW–MEDIUM | Fall back to deterministic digest; disable LLM flag; add validator; regenerate |
| Duplicate overlapping audits | LOW | Enable single-flight / CronJob Forbid; close duplicate runs; dedupe by `triggered_at`+scope |
| Leaked over-scoped token | HIGH | Rotate immediately; audit SCM/cloud access logs; shrink scopes; move to OIDC/IRSA |

## Pitfall-to-Phase Mapping

Agreed build order: **1 Trust & ops → 2 Live audit loop → 3 Leadership narrative**.

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| Sync HTTP = audit duration | Phase 1 | 202 + poll; handler unit test; no multi-minute blocking |
| Unauthenticated trigger / error leakage | Phase 1 | 401 without secret; opaque 500 bodies |
| Non-transactional persist / empty IDs / notifier coupling | Phase 1 | Postgres integration test; UUID PKs; Slack fail non-fatal |
| Unbounded concurrency skeleton | Phase 1 | Semaphore in agent; race tests for cancel/partial |
| Health/metrics/timeouts missing | Phase 1 | `/healthz`; server timeouts; basic run metrics |
| GitHub/AWS 429 fan-out | Phase 2 | Limiter + httptest 429 contract tests; partial dimension errors |
| FinOps false waste / tag gaps | Phase 2 | Confidence tiers; UNTAGGED before $; owner tag tests |
| Coverage parsers bypassed | Phase 2 | Fixture XML/JSON through agent path |
| Fake-healthy DORA | Phase 3 (guard early) | Unknown/omit semantics; agent asserts no fake green |
| Zero priority non-FinOps | Phase 3 | Mixed ranking fixture; digest sections |
| LLM ungrounded narrative | Phase 3 | Claim validator + deterministic fallback |
| Docs/manifest scale mismatch | Phase 1 docs + Phase 2 load | README accurate; optional 28-repo load test labeled as such |
| Test races / bad HTTP mocks | All phases (CI) | `go test -race`; httptest injection; no DefaultTransport swaps |

## Sources

- Codebase concerns: `.planning/codebase/CONCERNS.md`, `TDD.md`, `.planning/PROJECT.md` (2026-07-12) — **HIGH**
- GitHub REST best practices (avoid concurrent requests; Retry-After / rate-limit headers): https://docs.github.com/en/rest/using-the-rest-api/best-practices-for-using-the-rest-api — **HIGH**
- GitHub primary/secondary rate limits (concurrent + points/minute): https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api — **HIGH**
- Async request-reply (202 + poll): Microsoft Architecture Center asynchronous-request-reply pattern — **HIGH**
- Kubernetes CronJob `concurrencyPolicy`: https://kubernetes.io/docs/concepts/workloads/controllers/cron-jobs/ — **HIGH**
- AWS Compute Optimizer idle / rightsizing signals vs naive CPU averages: AWS Compute Blog + Compute Optimizer docs — **MEDIUM–HIGH**
- FinOps rightsizing false positives (percentile choice): industry FinOps write-ups (P95 vs burst workloads) — **MEDIUM**
- DORA definition / missing-data / gaming pitfalls: minware, Clouditive, Scrums DORA implementation articles — **MEDIUM**
- LLM summarization faithfulness / grounding failures: Vectara FaithJudge / RAG faithfulness literature — **MEDIUM**
- Go race detector: https://go.dev/doc/articles/race_detector — **HIGH**
- Go HTTP mocking: prefer `httptest.Server` + injected client over global `DefaultTransport` (Zus Health; Go testing guides) — **HIGH**

---
*Pitfalls research for: production-ready platform audit / FinOps agents*
*Researched: 2026-07-12*
