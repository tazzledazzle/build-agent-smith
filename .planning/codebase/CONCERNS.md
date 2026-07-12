# Codebase Concerns

**Analysis Date:** 2026-07-12

## Tech Debt

**Demo fixtures instead of live GitHub/AWS/Codecov clients:**
- Issue: Production entrypoint wires `demo.Sources{}` as the sole `agent.Dependencies` implementation; there is no real GitHub/GitLab API client, AWS Cost Explorer/CloudWatch/EC2 SDK, or Codecov fetcher.
- Files: `cmd/agent/main.go`, `internal/demo/sources.go`, `internal/agent/agent.go` (`Dependencies` interface)
- Why: Portfolio/demo path — deterministic fixtures let the HTTP agent and tests run without credentials.
- Impact: Audits always return the same canned estate (`payments-api`, idle EC2, etc.). README/TDD claims of 28-repo live measurement and $120K waste discovery are not reproducible from this binary.
- Fix approach: Implement real adapters behind `agent.Dependencies` (e.g. `internal/github`, `internal/aws`, `internal/codecov`), select via flag/env (`-sources=demo|live`), keep demo for tests.

**Postgres schema present; only in-memory store wired:**
- Issue: `migrations/001_schema.sql` defines `audit_runs`, `repo_scores`, `findings`, and `platform_health_current`, but `cmd/agent/main.go` persists via `store.Memory` only. No `database/sql` / pgx driver or Postgres `Store` implementation exists.
- Files: `migrations/001_schema.sql`, `internal/store/memory.go`, `internal/output/writer.go`, `cmd/agent/main.go`
- Why: Fast local demos without a database.
- Impact: Process restart loses all audit history; materialized view never updated; Grafana/dashboard path in `README.md` / `TDD.md` cannot work.
- Fix approach: Add `internal/store/postgres.go` implementing `output.Store`, run migrations on boot or via Makefile, thread DSN from env; keep `Memory` for unit tests.

**Slack notifier is log-only:**
- Issue: `logNotifier` in `cmd/agent/main.go` implements `output.Notifier` by printing to stdout (`log.Printf("slack digest:...")`). No Slack Web API client, webhook URL, or bot token handling.
- Files: `cmd/agent/main.go`, `internal/output/writer.go`
- Why: Avoid Slack credentials in the demo path.
- Impact: No real digests or critical alerts; operators must scrape logs. `Writer.Write` treats notifier failure as a hard error, so a future flaky Slack client would fail persistence-complete runs.
- Fix approach: Implement Slack Web API or incoming-webhook notifier; make digest failures non-fatal (log + continue) or retry with backoff; gate with `SLACK_WEBHOOK_URL`.

**Docs claim LangGraph; implementation is a Go goroutine orchestrator:**
- Issue: `README.md` and `TDD.md` describe a LangGraph (Python) agent with TypedDict state, LLM summarization, and Celery trade-offs. Runtime is Go (`internal/agent/agent.go` labeled “LangGraph-equivalent”) with supervisor + `sync.WaitGroup` workers — no LangGraph, no LLM calls, no Python.
- Files: `README.md`, `TDD.md`, `internal/agent/agent.go`, `go.mod`
- Why: Portfolio narrative retained while rewriting as a Go service.
- Impact: Onboarding/reviewers expect Python/LangGraph deps and LLM summarization; capacity planning (LLM tokens ~$0.15/run) does not apply. Misleading for `/gsd-plan-phase` if docs are trusted over code.
- Fix approach: Align docs to “Go supervisor/worker orchestrator (LangGraph-shaped topology)”; move LangGraph claims to historical appendix or remove; document that LLM summarization is not implemented.

**Coverage parsers unused by the agent path:**
- Issue: `coverage.ParseJaCoCoXML` and `coverage.ParseJSON` exist, but `agent.Run` only calls `deps.FetchCoverage` → `coverage.Analyze` with pre-parsed floats. Demo/stub deps never produce XML/JSON.
- Files: `internal/coverage/analyzer.go`, `internal/agent/agent.go`, `internal/demo/sources.go`
- Why: Parsing helpers prepared ahead of live Codecov/CI artifact integration.
- Impact: Live wiring could bypass parsers entirely; JaCoCo/Codecov formats are untested end-to-end through the agent.
- Fix approach: Have live coverage deps fetch raw reports and call parsers before `Analyze`; add agent-level tests with fixture XML/JSON.

**Repo standardization rubric vs TDD scope:**
- Issue: Scanner checks five file-presence dimensions only. TDD also lists branch protection, required reviewers, and template drift via cosine similarity on YAML — none implemented.
- Files: `internal/reposcanner/scanner.go`, `TDD.md` (Repo Scanner Node section)
- Why: File-presence is enough for a demo scoring story.
- Impact: Standardization scores understate real org drift; leadership dashboards would miss governance gaps.
- Fix approach: Extend `FileChecker` / add GitHub branch-protection API checks; implement template-diff scoring or drop those claims from docs.

**Manifest size vs narrative (5 vs 28 repos):**
- Issue: `configs/repos.json` lists 5 repos; README/TDD repeatedly cite 28 repositories and ~560 GitHub API calls per full audit.
- Files: `configs/repos.json`, `README.md`, `TDD.md`, `internal/config/repos_manifest_test.go`
- Why: Small fixture estate for demos/tests.
- Impact: Performance and capacity claims are unverified; smoke/e2e never exercise 28-repo fan-out.
- Fix approach: Either expand the manifest for load tests or qualify docs as “designed for ~28; demo ships 5.”

## Known Bugs

**DORA composites look healthy when delivery metrics are never set:**
- Symptoms: Every live agent run produces `DoraCompositeScore` around ~7.5 for repos even though no deploy/lead-time/CFR/MTTR data is collected.
- Trigger: Run a full audit via `agent.Run` or `POST /audit/trigger` with demo or stub deps; inspect `state.DoraScores` / `RepoScores`.
- Files: `internal/agent/agent.go` (only sets CICD/standardization/coverage fields on `RepoMetrics`), `internal/aggregator/aggregator.go` (`computeDORA`)
- Workaround: Ignore DORA scores until real inputs exist; treat them as placeholders.
- Root cause: Zero-valued `LeadTimeDays`, `ChangeFailureRate`, and `MTTRHours` map to perfect (10) scores; only `DeploysPerWeek=0` correctly scores low. Composite ≈ `(0+10+10+10)/4 = 7.5`.
- Fix: Require explicit metrics (or use sentinel “unknown”); do not score unset dimensions; or omit DORA from output until populated.

**Finding rows lack IDs (and often RepoName for FinOps):**
- Symptoms: `domain.Finding.ID` is always empty after audits. FinOps findings omit `RepoName` (resource-centric only). Schema requires `findings.id UUID PRIMARY KEY`.
- Trigger: Inspect findings from any worker; attempt insert into Postgres per `migrations/001_schema.sql`.
- Files: `internal/cicd/auditor.go`, `internal/coverage/analyzer.go`, `internal/reposcanner/scanner.go`, `internal/finops/analyzer.go`, `internal/aggregator/aggregator.go`, `migrations/001_schema.sql`
- Workaround: In-memory store accepts empty IDs; Postgres path will fail until fixed.
- Root cause: Workers never assign `uuid.NewString()` (or similar); FinOps `cloudFinding` does not set `RepoName`.
- Fix: Assign IDs in workers or aggregator; map cloud resources to owning service via tags (`service` tag → `RepoName`) where possible.

**Non-FinOps findings all rank at priority 0:**
- Symptoms: CI/CD, coverage, and standardization findings sort below any positive-cost FinOps finding and are unordered among themselves by priority (all `PriorityScore == 0`).
- Trigger: Full audit with demo sources; compare digests/`state.Findings` order.
- Files: `internal/domain/types.go` (`PriorityScore`), gap finders in `internal/cicd/auditor.go`, `internal/coverage/analyzer.go`, `internal/reposcanner/scanner.go`
- Workaround: Manually triage by severity/type.
- Root cause: Those finders set `EstimatedCostUSD: 0`, so `cost * severity / effort == 0`.
- Fix: Use severity-weighted baseline cost, effort-only ranking for non-cost findings, or a composite key `(severity, type, effort)`.

**HTTP trigger may skip JSON body when `Content-Length` is 0 or unknown:**
- Symptoms: Request body ignored; scope silently defaults to `full`.
- Trigger: Clients that omit `Content-Length` or send `Content-Length: 0` with a body (some proxies/chunked edge cases). Handler only decodes when `r.ContentLength != 0`.
- Files: `internal/api/handler.go`
- Workaround: Always send explicit `Content-Length` and non-empty JSON (smoke script does).
- Root cause: Guard uses `ContentLength != 0` instead of attempting decode / using `http.MaxBytesReader`.
- Fix: Always `json.NewDecoder(r.Body).Decode` for POST; treat EOF as empty body; limit body size with `MaxBytesReader`.

## Security Considerations

**Unauthenticated `/audit/trigger`:**
- Risk: Anyone who can reach the port can start full audits (CPU/goroutine load) and, once live sources exist, burn GitHub/AWS quota or exfiltrate findings via response metadata.
- Files: `internal/api/handler.go`, `cmd/agent/main.go`
- Current mitigation: None (local bind default `:8080`; no auth middleware, no network policy in-repo).
- Recommendations: Shared secret / mTLS / IAM auth gateway; bind to localhost by default; rate-limit; separate webhook HMAC verification when GitHub webhooks are added.

**Error text returned to clients:**
- Risk: `http.Error(w, err.Error(), 500)` can leak internal paths, manifest details, or upstream API errors once live clients exist.
- Files: `internal/api/handler.go`
- Current mitigation: Demo errors are mostly benign planner messages.
- Recommendations: Log full error server-side; return opaque `audit failed` to clients.

**Secret-hygiene regex is heuristic only:**
- Risk: False negatives (obfuscated secrets, base64, vault refs mis-scored) and false positives; not a substitute for secret scanning tools. Scoring awards full SecretHygiene (3) when no regex hits.
- Files: `internal/cicd/auditor.go` (`secretPatterns`, `scoreSecretHygiene`)
- Current mitigation: Basic patterns for key=value, `AKIA…`, PEM headers.
- Recommendations: Integrate gitleaks/trufflehog-style scanning; never log matched secret substrings.

**No TLS or security headers on HTTP server:**
- Risk: Cleartext digests/findings on the wire if exposed beyond localhost.
- Files: `cmd/agent/main.go` (`http.Server`)
- Current mitigation: Assumed local/demo deployment.
- Recommendations: Terminate TLS at ingress; add `ReadTimeout`/`WriteTimeout`/`IdleTimeout` (only `ReadHeaderTimeout` is set today).

## Performance Bottlenecks

**Unbounded per-repo worker fan-out:**
- Problem: For each repo, up to three goroutines (`repo_scanner`, `cicd_auditor`, `coverage_analyzer`) plus one FinOps worker — no worker pool or concurrency limit.
- Files: `internal/agent/agent.go`
- Measurement: Not benchmarked in-repo. At TDD’s 28 repos → ~84+ concurrent workers; with live API latency this will amplify rate-limit pressure (~560 GitHub calls claimed in `TDD.md`).
- Cause: `runWorker` spawns a goroutine per task with only `sync.WaitGroup` coordination.
- Improvement path: Semaphore/`errgroup` with bounded parallelism (e.g. 8–16); batch GitHub GraphQL; cache file/YAML fetches per repo.

**Synchronous HTTP request = full audit duration:**
- Problem: `Handler.trigger` blocks until `runner.Run` + `writer.Write` complete; no async job ID / poll API.
- Files: `internal/api/handler.go`, `cmd/agent/main.go` (`agentRunner`)
- Measurement: Demo estate is fast (sub-second). TDD targets &lt;15 min for 28 repos — HTTP clients and load balancers will time out long before that without async design.
- Cause: Request-scoped orchestration.
- Improvement path: Accept trigger → enqueue → return `202` + `audit_run_id`; run workers in background; add `GET /audit/runs/{id}`.

**In-memory store grows without bound:**
- Problem: `store.Memory` appends every run/score/finding forever for the process lifetime.
- Files: `internal/store/memory.go`
- Measurement: ~200–400 findings/run per TDD; weekly runs → memory creep in long-lived pods.
- Cause: Demo store has no retention/TTL.
- Improvement path: Postgres persistence with retention policy; if Memory kept for demos, cap history or clear between runs.

**Repo scanner sequential path checks:**
- Problem: Each of 5 dimensions may probe multiple candidate paths via `HasFile` sequentially.
- Files: `internal/reposcanner/scanner.go`
- Measurement: Worst case several round-trips per dimension per repo against a live GitHub API.
- Cause: Nested loops with early break only on hit.
- Improvement path: Single tree/contents API call per repo; parallelize path checks carefully under rate limits.

## Fragile Areas

**Agent worker coordination and partial failure semantics:**
- Files: `internal/agent/agent.go`
- Why fragile: Many goroutines mutate shared `findings`/`metrics`/`errs` under one mutex; status becomes `PARTIAL_AUDIT` if any worker errors, but aggregation still runs on partial metrics. Context cancel is checked only after `wg.Wait`, so shutdown waits for all workers.
- Common failures: One bad YAML parse marks partial audit while other repos look complete; hard to attribute which repo failed (`AuditError.Repo` is never set).
- Safe modification: Add tests for mixed success/failure; set `AuditError.Repo`; consider `errgroup` + per-repo aggregation; check `ctx` inside workers and document PARTIAL semantics.
- Test coverage: Unit/e2e cover happy paths (`internal/agent/agent_test.go`, `e2e_test.go`); limited cancellation/partial-failure cases.

**CI/CD maturity heuristics (string/regex over structure):**
- Files: `internal/cicd/auditor.go`
- Why fragile: Scoring mixes YAML structure and substring checks (`actions/cache`, `:latest`, job name contains `test`). Easy to game or false-score unconventional pipelines.
- Common failures: Test gate awarded for a job named `test` without real tests; artifact pinning defaults to 1 when no `:latest` and no pins.
- Safe modification: Expand golden YAML fixtures in `internal/cicd/auditor_test.go` before changing rubrics; keep dimension max scores documented next to code.
- Test coverage: Solid unit tests for common GH Actions shapes; GitLab and exotic workflows thinner.

**Output writer non-transactional multi-save:**
- Files: `internal/output/writer.go`, `internal/store/memory.go`
- Why fragile: `SaveAuditRun` → `SaveRepoScores` → `SaveFindings` → notifier; failure mid-way leaves partial persisted state (especially once Postgres exists).
- Common failures: Findings missing for a run that appears in `audit_runs`; Slack failure after DB writes returns error to API despite data saved.
- Safe modification: Introduce transactional `Store.SaveAudit(state)` or compensate on failure; decouple notifier errors from persistence success.
- Test coverage: `internal/output/writer_test.go` happy path only.

**Demo sources tightly coupled to fixture repo names:**
- Files: `internal/demo/sources.go`, `configs/repos.json`
- Why fragile: Hard-coded switches on `payments-api`, `identity-svc`, `platform-infra`. Renaming manifest entries silently changes scores/findings.
- Common failures: New repos always get “immature” defaults; tests asserting waste assume demo inventory.
- Safe modification: Keep manifest and demo switches in sync; prefer table-driven fixtures keyed by repo name in one place.
- Test coverage: E2E and live API tests depend on this coupling (`internal/agent/e2e_test.go`, `internal/api/live_trigger_test.go`).

## Scaling Limits

**In-process Memory store + single binary:**
- Current capacity: Suitable for local demo (5 repos, one process).
- Limit: No horizontal scale, no shared state across replicas; duplicate triggers = duplicate in-memory histories.
- Symptoms at limit: Lost history on restart; inconsistent digests across pods if replicated.
- Scaling path: Postgres + single-flight / lease per audit scope; queue-based workers.

**Goroutine fan-out under live API rate limits:**
- Current capacity: Fine for demo (no network).
- Limit: GitHub 5,000 req/hr (TDD); unbounded concurrency will 429 long before CPU saturates.
- Symptoms at limit: Worker errors → `PARTIAL_AUDIT`; incomplete findings.
- Scaling path: Rate limiter + concurrency cap; conditional requests/ETags; GraphQL batching.

**HTTP server timeouts incomplete:**
- Current capacity: Demo requests complete quickly.
- Limit: No `WriteTimeout`/`ReadTimeout`; long audits hold connections indefinitely.
- Symptoms at limit: Connection exhaustion under repeated `/audit/trigger` while audits run.
- Scaling path: Async jobs + short HTTP timeouts; health/readiness endpoints (none today).

## Dependencies at Risk

**Go toolchain `go 1.26.2`:**
- Risk: `go.mod` pins a very new Go version; Docker base `golang:1.26-alpine` must match. Environments without that toolchain cannot build.
- Files: `go.mod`, `Dockerfile`
- Impact: CI/contributor setup friction; image pull failures if tag unavailable.
- Migration plan: Confirm intended version; document required Go in README; pin digest in Dockerfile.

**Minimal third-party surface (uuid + yaml.v3 only):**
- Risk: Low supply-chain risk today, but adding AWS/GitHub/Slack SDKs will expand attack/update surface quickly with no existing dependency policy.
- Files: `go.mod`, `go.sum`
- Impact: Future live integrations need deliberate versioning and vulnerability scanning.
- Migration plan: Prefer official AWS SDK v2 / go-github; pin versions; add `govulncheck` to `Makefile`.

**`make lint` assumes golangci-lint:**
- Risk: No `.golangci.yml` in repo; `Makefile` `lint` target fails where the binary is missing.
- Files: `Makefile`
- Impact: Inconsistent local lint vs `go vet`.
- Migration plan: Vendor config or document install; add lint to CI when CI exists (no `.github/workflows` detected).

## Missing Critical Features

**Live external data sources:**
- Problem: No GitHub/GitLab, AWS, or Codecov clients.
- Current workaround: `demo.Sources` fixtures.
- Blocks: Real platform audits, FinOps right-sizing from production accounts, PR incremental accuracy.
- Implementation complexity: High (auth, pagination, rate limits, multi-provider).

**Durable Postgres persistence + MV refresh:**
- Problem: Schema exists; no runtime migration, insert path, or `REFRESH MATERIALIZED VIEW`.
- Current workaround: In-memory slices for demos/tests.
- Blocks: Grafana dashboard, historical trends, append-only findings store described in TDD.
- Implementation complexity: Medium.

**Schedulers and webhooks:**
- Problem: No cron CronJob manifests, no GitHub webhook handler, no Slack slash-command path — only manual `POST /audit/trigger`.
- Current workaround: `make run` / `scripts/test-audit-trigger.sh`.
- Blocks: Weekly autonomous audits and post-merge incremental Path 3 in `TDD.md` (including PR comments / score deltas).
- Implementation complexity: Medium.

**LLM summarization / narrative prioritization:**
- Problem: TDD chooses LLM for summarization only; no LLM client or prompts in code.
- Current workaround: Deterministic `buildDigest` string in `internal/output/writer.go`.
- Blocks: Narrative executive digests claimed in design trade-offs.
- Implementation complexity: Medium (optional for MVP).

**Health/metrics/observability:**
- Problem: No `/healthz`, no Prometheus metrics (`node_execution_duration_ms` etc. in TDD), no structured `audit_run_id` log fields beyond default `log`.
- Current workaround: stdout logs.
- Blocks: K8s probes, PagerDuty SLO alerting, node-level ops.
- Implementation complexity: Low–medium.

## Test Coverage Gaps

**No live adapter or contract tests:**
- What's not tested: Real GitHub/AWS/Codecov HTTP behavior, auth failures, pagination, 429 retry.
- Files: Only `internal/demo/sources.go` + stubs in `internal/agent/agent_test.go`
- Risk: First live integration will fail in production shapes not covered by fixtures.
- Priority: High (when leaving demo mode)
- Difficulty to test: Need httptest fixtures / recorded responses (vcr).

**Postgres store and migrations:**
- What's not tested: Schema apply, FK integrity, empty finding IDs, materialized view query.
- Files: `migrations/001_schema.sql` (no Go tests)
- Risk: First Postgres cutover breaks on UUID/PK and partial writes.
- Priority: High (before enabling Postgres)
- Difficulty to test: Requires testcontainers or embedded Postgres.

**Auth, timeouts, and abuse of `/audit/trigger`:**
- What's not tested: Missing auth, concurrent triggers, cancelled request mid-audit, body size limits.
- Files: `internal/api/handler_test.go`, `internal/api/live_trigger_test.go`
- Risk: Production exposure issues go unnoticed.
- Priority: Medium
- Difficulty to test: Straightforward once middleware exists.

**DORA and priority ranking realism:**
- What's not tested: Agent path never asserts non-placeholder DORA; priority ranking with zero-cost findings.
- Files: `internal/aggregator/aggregator_test.go` (unit-only with hand-fed metrics), `internal/agent/*`
- Risk: Misleading scores ship in digests/APIs.
- Priority: High
- Difficulty to test: Low — add assertions that unset metrics do not yield high composites.

**No CI workflow in-repo:**
- What's not tested automatically on push: `go test ./...`, race detector, smoke script.
- Files: No `.github/workflows/*`; `Makefile` defines `test`/`smoke` locally only.
- Risk: Regressions land unnoticed.
- Priority: Medium
- Difficulty to test: Low — add GitHub Actions running `make test` and optionally `make smoke`.

---

*Concerns audit: 2026-07-12*
*Update as issues are fixed or new ones discovered*
