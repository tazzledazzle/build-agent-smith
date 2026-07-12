# External Integrations

**Analysis Date:** 2026-07-12

## APIs & External Services

**Payment Processing:**
- Not applicable

**Email/SMS:**
- Not applicable

**External APIs (designed; not live in current binary):**

Designed integrations are abstracted behind `agent.Dependencies` in `internal/agent/agent.go` and `output.Store` / `output.Notifier` in `internal/output/writer.go`. The running process wires **demo fixtures**, not network clients (`cmd/agent/main.go` → `demo.Sources{}`, `store.Memory`, `logNotifier`).

- GitHub / GitLab APIs - Repo file presence and CI pipeline YAML fetch
  - Integration method: Planned REST via `Dependencies.HasFile` / `FetchPipelineYAML`; today `internal/demo/sources.go` returns hard-coded fixtures
  - Auth (design, `TDD.md`): read-only tokens (`repo:read`, `ci:read`); Secrets Manager — no env vars in code
  - Manifest providers: `"github"` | `"gitlab"` in `configs/repos.json` / `domain.RepoConfig.Provider`

- Codecov / JaCoCo-style coverage - Line/branch coverage inputs
  - Integration method: Planned fetch into `agent.CoverageInput`; today `demo.Sources.FetchCoverage`
  - Parsers ready for inbound payloads: Codecov-style JSON and JaCoCo-style XML in `internal/coverage/analyzer.go` (`ParseJSON`, XML types)
  - Auth (design): Codecov API token via Secrets Manager — not implemented

- AWS Cost Explorer + CloudWatch / EC2·RDS·EBS inventory - FinOps waste detection
  - Integration method: Planned SDK calls into `finops.Inventory`; today `demo.Sources.FetchCloudInventory`
  - Analysis is local in `internal/finops/analyzer.go` (idle EC2, over-provisioned RDS, orphaned EBS, untagged resources)
  - Auth (design, `TDD.md`): IAM role with ReadOnlyAccess + `CostExplorer:GetCostAndUsage` — no AWS SDK in `go.mod`

- Slack - Weekly / on-demand audit digests
  - Integration method: `output.Notifier.PostDigest`; production would call Slack API
  - Current: `logNotifier` in `cmd/agent/main.go` logs digest text (`log.Printf("slack digest:\n%s", text)`)
  - Auth (design): Slack bot token via Secrets Manager — not implemented

- LLM provider - Summarization / narrative (design only in `TDD.md`)
  - Not present in Go module or runtime wiring
  - Auth (design): API key via AWS Secrets Manager

## Data Storage

**Databases:**
- PostgreSQL - Designed persistence for `audit_runs`, `repo_scores`, `findings`, and materialized view `platform_health_current`
  - Schema: `migrations/001_schema.sql`
  - Connection: Not configured — no `database/sql` driver or `DATABASE_URL` usage
  - Client: Interface `output.Store` in `internal/output/writer.go`
  - Runtime adapter: in-process `store.Memory` in `internal/store/memory.go` (tests and local demos)

**File Storage:**
- Local filesystem only
  - Repo manifest: `configs/repos.json` (copied into Docker image under `/app/configs/`)
  - Coverage profile artifact: `coverage.out` (gitignored; produced by `make test`)
  - Built binary: `bin/agent` (gitignored)

**Caching:**
- None (no Redis or similar)

## Authentication & Identity

**Auth Provider:**
- None for the HTTP API — `POST /audit/trigger` is unauthenticated (`internal/api/handler.go`)

**OAuth Integrations:**
- Not applicable

**Secrets (design vs. current):**
- Design (`TDD.md`): AWS Secrets Manager for GitHub/GitLab tokens, AWS credentials, LLM API key; no credentials in code or env
- Current codebase: no secret loading, no `.env` / `.env.example`, no Secrets Manager client

## Monitoring & Observability

**Error Tracking:**
- None (no Sentry/PagerDuty SDK). Design mentions PagerDuty if audit exceeds 30-minute SLO (`TDD.md`)

**Analytics:**
- None

**Logs:**
- Stdlib `log` package — listen/shutdown messages and Slack digest stub (`cmd/agent/main.go`)
- No structured JSON logging, metrics exporters, or OpenTelemetry

**Dashboards (design):**
- Grafana over Postgres materialized view `platform_health_current` (`migrations/001_schema.sql`, `TDD.md` / `README.md`) — not deployed from this repo

## CI/CD & Deployment

**Hosting:**
- Docker image (`Dockerfile`) — static binary, port 8080, default flags `-addr :8080 -manifest configs/repos.json`
- Design target: Kubernetes CronJob for weekly full audits + always-on HTTP for triggers (`TDD.md`)
- No Kubernetes manifests or compose files in repo

**CI Pipeline:**
- None in-repo (no `.github/workflows/`)
- Local quality gates via `Makefile`: `test`, `vet`, `lint`, `fmt`, `smoke`
- Smoke integration: `scripts/test-audit-trigger.sh` builds binary, boots server, curls `/audit/trigger`

## Environment Configuration

**Development:**
- Required env vars: none for the agent binary
- Optional smoke-script vars: `AUDIT_ADDR`, `AUDIT_MANIFEST`, `AUDIT_BIN`, `AUDIT_READY_TIMEOUT`
- CLI: `-addr`, `-manifest`
- Mock/stub services: `internal/demo/sources.go` (GitHub/GitLab/Codecov/AWS fixtures), `store.Memory`, `logNotifier`
- Secrets location: Not applicable for current local path

**Staging:**
- Not detected (no staging config)

**Production:**
- Secrets management (design): AWS Secrets Manager
- Failover/redundancy: Not implemented in this repo
- Container entrypoint ships with demo data sources until live adapters replace `demo.Sources` / `store.Memory`

## Webhooks & Callbacks

**Incoming:**
- HTTP trigger endpoint: `POST /audit/trigger` (`internal/api/handler.go`)
  - Body: optional JSON `{"scope":"full"|"incremental"|"finops_only","repo":"<name>"}` (`api.TriggerRequest`)
  - Default scope: `full` when body empty
  - Incremental requires `repo`; returns 400 otherwise
  - Designed consumers (`TDD.md` / `README.md`): weekly cron, Slack command, GitHub PR-merge webhook — no signature verification or GitHub webhook handler implemented
  - Response: JSON `audit_run_id`, `status`, `finding_count`, `scope` (`api.TriggerResponse`)

**Outgoing:**
- Slack digest via `Notifier.PostDigest` after persist (`internal/output/writer.go`) — currently stdout log only
- Design: PR comment on incremental post-merge audits (`TDD.md`) — not implemented
- No outbound webhook clients in code

## Integration Seams (for wiring real services)

| Seam | Interface | Current impl | Replace with |
|------|-----------|--------------|--------------|
| Repo / CI / coverage / cloud fetch | `agent.Dependencies` (`internal/agent/agent.go`) | `demo.Sources` (`internal/demo/sources.go`) | GitHub/GitLab + Codecov + AWS SDK clients |
| Persist findings | `output.Store` (`internal/output/writer.go`) | `store.Memory` (`internal/store/memory.go`) | PostgreSQL driver implementing schema in `migrations/001_schema.sql` |
| Slack digest | `output.Notifier` (`internal/output/writer.go`) | `logNotifier` (`cmd/agent/main.go`) | Slack Web API client |

---

*Integration audit: 2026-07-12*
*Update when adding/removing external services*
