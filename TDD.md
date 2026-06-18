# Platform Maturity Audit Agent — Technical Design Document

**Author:** Terence  
**Date:** 2026-06-18  
**Status:** Reference / Portfolio  
**Tags:** LangGraph, Platform Engineering, FinOps, DORA, Developer Productivity

---

## Problem Statement

Engineering organizations that scale infrastructure faster than they build visibility accumulate invisible technical debt: unstandardized CI/CD pipelines, inconsistent test coverage, ungoverned cloud resources, and no continuous signal on platform health. At Invisible Technologies, 28 repositories across 310+ microservices had no unified view of infrastructure standardization, CI/CD maturity, or cloud utilization. Leadership had no way to answer: *"Are we operationally healthy?"* without manual, ad-hoc audits that went stale the moment they were completed.

---

## Goals & Non-Goals

### Goals
- Continuously measure infrastructure standardization, CI/CD maturity, and test coverage across all 28 repositories
- Surface cloud resource utilization patterns to FinOps for right-sizing decisions
- Provide engineering leadership a unified, always-current platform health dashboard
- Produce actionable findings, not just metrics — ranked by cost impact and remediation effort
- Operate autonomously as a scheduled agent; require zero manual audit effort per cycle

### Non-Goals
- Auto-remediate findings (agent identifies; humans act)
- Replace purpose-built APM (Datadog, Grafana) for runtime observability
- Audit application-layer code quality (separate concern)
- Cover external vendor/SaaS dependencies

---

## Background & Context

Manual platform audits were run quarterly and required 2–3 engineer-weeks of effort. By the time findings were presented, the state had drifted. Cloud costs were opaque: FinOps had billing data but no attribution to specific service patterns or ownership. The 28-repo estate had grown organically with no enforced CI/CD template, resulting in wildly inconsistent pipeline structures (some repos had no tests at all, others had 3+ competing lint configurations).

The system was designed to run inside existing GitHub/GitLab + AWS infrastructure with no new vendor dependencies beyond the LLM provider.

---

## Proposed Design

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     Orchestration Layer                          │
│                                                                  │
│   [Scheduler / Trigger]  →  [LangGraph Audit Agent]             │
│          (cron / webhook)         │                              │
│                            ┌──────┴──────────────┐              │
│                            │   Supervisor Node    │              │
│                            │  (plan + route)      │              │
│                            └──────────────────────┘              │
│                      ↙         ↓          ↓         ↘           │
│              [Repo      [CI/CD      [Coverage    [Cloud         │
│              Scanner]   Auditor]    Analyzer]    FinOps]        │
│                Node]      Node]       Node]       Node]         │
└─────────────────────────────────────────────────────────────────┘
        ↓               ↓               ↓               ↓
  [GitHub API]   [Pipeline Configs]  [Coverage     [AWS Cost
  [GitLab API]   [YAML Parser]       Reports]      Explorer API]
                                     [Codecov API] [CloudWatch]
                                                   [EC2 / RDS
                                                    inventory]
        ↓               ↓               ↓               ↓
┌─────────────────────────────────────────────────────────────────┐
│                    Findings Aggregator Node                      │
│         (deduplicate, rank by impact, generate DORA scores)     │
└─────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────┐
│                     Output Layer                                 │
│   [PostgreSQL findings store]   [Slack digest]   [Dashboard]    │
└─────────────────────────────────────────────────────────────────┘
```

### LangGraph Node Topology

```
START
  │
  ▼
[supervisor]          ← Plans audit scope, decides which workers to invoke
  │
  ├──► [repo_scanner]        ← Clones/scans each repo for CI file presence,
  │                             Dockerfile standards, linting configs, 
  │                             branch protection, required reviewers
  │
  ├──► [cicd_auditor]        ← Parses .github/workflows/*.yml or .gitlab-ci.yml,
  │                             scores against maturity rubric:
  │                             (caching, artifact pinning, parallelism, 
  │                              test gates, deployment controls)
  │
  ├──► [coverage_analyzer]   ← Fetches coverage reports from Codecov / JaCoCo,
  │                             flags repos below threshold (<60% line coverage),
  │                             correlates with change frequency
  │
  └──► [cloud_finops]        ← Calls AWS Cost Explorer + EC2/RDS DescribeInstances,
                                correlates service owner tags with utilization metrics,
                                flags: idle instances, over-provisioned RDS, 
                                orphaned EBS volumes, untagged resources
        │
        ▼
  [findings_aggregator]      ← Merges all node outputs, deduplicates,
        │                       applies DORA scoring model, ranks by:
        │                       (cost_impact × severity × remediation_ease)
        │
        ▼
  [output_writer]            ← Persists to Postgres, triggers Slack digest,
        │                       updates dashboard materialized view
        │
        ▼
END
```

### Key Components

#### 1. Supervisor Node
The LangGraph orchestrator. Reads the list of repositories from a config manifest, partitions work into parallel sub-tasks, and routes to appropriate worker nodes based on the scope of the current audit cycle (full vs. incremental).

**State schema (Python TypedDict):**
```python
class AuditState(TypedDict):
    repos: List[RepoConfig]
    scope: Literal["full", "incremental", "finops_only"]
    findings: List[Finding]
    dora_scores: Dict[str, DoraScore]
    audit_run_id: str
    errors: List[AuditError]
```

#### 2. Repo Scanner Node
- Authenticates with GitHub/GitLab APIs via token
- For each repo: checks for presence of CI config, Dockerfile, `.pre-commit-config.yaml`, `CODEOWNERS`
- Scores standardization: 0–5 rubric per dimension
- Flags drift from org-wide templates (computed via cosine similarity on parsed YAML structure)

#### 3. CI/CD Auditor Node
Parses pipeline YAML and scores against a 6-dimension maturity rubric:

| Dimension | Criteria | Max Score |
|---|---|---|
| Caching | Dependency cache configured | 1 |
| Test gate | Tests run before deploy | 2 |
| Parallelism | Jobs split across runners | 1 |
| Artifact pinning | No `latest` tags | 1 |
| Deploy controls | Manual approval on prod | 2 |
| Secret hygiene | No hardcoded secrets (regex scan) | 3 |

Total: 0–10 per repo. Orgs scoring <6 flagged for remediation.

#### 4. Coverage Analyzer Node
- Fetches coverage XML/JSON from CI artifacts or Codecov API
- Correlates with commit frequency (high-churn, low-coverage = high risk)
- Produces per-repo coverage vector: `{line_pct, branch_pct, change_risk_score}`

#### 5. Cloud FinOps Node
- Calls `AWS Cost Explorer` for trailing 30-day spend by service + tag
- Calls `EC2 DescribeInstances` for CPU utilization via CloudWatch metrics
- Calls `RDS DescribeDBInstances` for FreeableMemory, CPUUtilization
- Flags:
  - EC2 instances with CPU <5% over 14 days → `IDLE`
  - RDS instances with <20% average utilization → `OVER_PROVISIONED`
  - EBS volumes with no attachment → `ORPHANED`
  - Resources missing `team` or `service` tags → `UNTAGGED`

Produces: `{resource_id, resource_type, flag, estimated_monthly_waste_usd, owner_tag}`

#### 6. Findings Aggregator Node
- Merges outputs from all worker nodes
- Assigns DORA-aligned maturity scores per repo:
  - Deployment Frequency, Lead Time for Changes, Change Failure Rate proxy, MTTR proxy
- Ranks findings by: `cost_impact_usd * severity_weight / remediation_hours_estimate`
- Produces executive summary (top 10 findings) + full findings list

### Data Model

```sql
-- Core findings store
CREATE TABLE audit_runs (
    id UUID PRIMARY KEY,
    triggered_at TIMESTAMPTZ,
    scope TEXT,
    repo_count INT,
    total_findings INT,
    total_estimated_waste_usd NUMERIC(10,2),
    completed_at TIMESTAMPTZ
);

CREATE TABLE repo_scores (
    audit_run_id UUID REFERENCES audit_runs(id),
    repo_name TEXT,
    standardization_score INT,   -- 0-5
    cicd_maturity_score INT,      -- 0-10
    coverage_line_pct NUMERIC(5,2),
    coverage_risk_score NUMERIC(5,2),
    dora_composite_score NUMERIC(5,2),
    PRIMARY KEY (audit_run_id, repo_name)
);

CREATE TABLE findings (
    id UUID PRIMARY KEY,
    audit_run_id UUID REFERENCES audit_runs(id),
    repo_name TEXT,
    finding_type TEXT,            -- 'cicd_gap' | 'coverage_risk' | 'cloud_waste' | 'standardization'
    severity TEXT,                -- 'critical' | 'high' | 'medium' | 'low'
    title TEXT,
    description TEXT,
    estimated_cost_usd NUMERIC(10,2),
    remediation_effort_hours INT,
    priority_score NUMERIC(10,4), -- cost * severity / effort
    resource_id TEXT,             -- for cloud findings
    owner_tag TEXT,
    status TEXT DEFAULT 'open',   -- 'open' | 'acknowledged' | 'resolved'
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Materialized view for dashboard
CREATE MATERIALIZED VIEW platform_health_current AS
SELECT
    r.repo_name,
    r.cicd_maturity_score,
    r.coverage_line_pct,
    r.dora_composite_score,
    COUNT(f.id) FILTER (WHERE f.severity = 'critical') AS critical_findings,
    SUM(f.estimated_cost_usd) AS total_waste_usd
FROM repo_scores r
JOIN audit_runs a ON r.audit_run_id = a.id
LEFT JOIN findings f ON f.audit_run_id = a.id AND f.repo_name = r.repo_name
WHERE a.id = (SELECT id FROM audit_runs ORDER BY completed_at DESC LIMIT 1)
GROUP BY r.repo_name, r.cicd_maturity_score, r.coverage_line_pct, r.dora_composite_score;
```

### Critical Paths

**Path 1: Scheduled Full Audit**
```
Cron trigger (weekly)
  → Supervisor loads repo manifest (28 repos)
  → Fan-out: 4 worker nodes run in parallel per repo batch
  → Findings aggregator merges ~280 signals
  → Output writer: Postgres insert + Slack digest + dashboard refresh
  → Total runtime target: <15 minutes for 28 repos
```

**Path 2: On-Demand FinOps Scan**
```
Manual trigger (Slack command or API call)
  → Supervisor sets scope=finops_only
  → Only cloud_finops node runs
  → Aggregator produces cost-only findings
  → Output to Slack thread + findings table
  → Runtime target: <3 minutes
```

**Path 3: Post-Merge Incremental**
```
GitHub webhook (PR merged to main)
  → Supervisor sets scope=incremental, single repo
  → repo_scanner + cicd_auditor + coverage_analyzer run (no FinOps)
  → Findings delta computed vs. last full audit
  → PR comment posted with score delta
  → Runtime target: <90 seconds
```

---

## Trade-offs Considered

| Decision | Option A (Chosen) | Option B | Rationale |
|---|---|---|---|
| Orchestration | LangGraph (stateful DAG) | Celery + custom orchestration | LangGraph provides native conditional routing, state persistence, and parallel node execution without hand-rolling task queuing |
| LLM role | Summarization + prioritization only | Full agentic (LLM drives all decisions) | Deterministic code for metrics collection; LLM only where judgment adds value (finding narrative, priority ranking explanation) |
| Storage | PostgreSQL + materialized view | Elasticsearch | Structured findings with relational joins are a better fit than full-text search; Postgres sufficient at this data volume |
| Cloud audit | AWS SDK direct | Third-party FinOps tool (CloudHealth) | No additional vendor; SDK gives full control; tool would require procurement and onboarding |
| Scheduling | Cron-based (weekly full + event-driven incremental) | Continuous polling | Cron is operationally simple; event-driven incremental covers the latency-sensitive case |
| Parallel execution | LangGraph parallel node fan-out | Sequential per-repo | 28 repos × 4 audit dimensions = too slow sequentially; parallel fan-out targets <15 min total |

---

## Operational Considerations

### Deployment
- Containerized (Docker), deployed as a long-running Kubernetes CronJob for scheduled runs
- Exposes `/audit/trigger` REST endpoint for on-demand scans and webhook integration
- Secrets managed via AWS Secrets Manager (GitHub token, AWS credentials, LLM API key)

### Observability
- Each audit run emits structured logs with `audit_run_id` for full trace correlation
- Metrics emitted per node: `node_execution_duration_ms`, `node_error_count`, `findings_produced_count`
- Alerting: PagerDuty alert if audit run fails to complete within 30-minute SLO
- Dashboard: Grafana panels over the `platform_health_current` materialized view

### Rollback
- Findings are immutable, append-only — no rollback needed for data
- Agent code deployments use blue/green; old image retained for 2 weeks
- If a node produces malformed output, findings aggregator skips that dimension and logs `PARTIAL_AUDIT` status

---

## Security & Compliance

- GitHub/GitLab tokens scoped to read-only (`repo:read`, `ci:read`)
- AWS credentials use IAM role with `ReadOnlyAccess` + `CostExplorer:GetCostAndUsage` only
- No credentials stored in code or environment variables — all via Secrets Manager
- Findings data classified as internal (contains repo structure and cloud resource details)
- Audit logs retained 90 days per policy

---

## Capacity Planning

| Dimension | Estimate |
|---|---|
| Repos | 28 (stable) |
| Findings per full audit | ~200–400 rows |
| Findings store growth | ~20K rows/year (weekly audits × ~350 findings) |
| DB storage | <1GB/year — trivial |
| API calls per full audit | ~560 GitHub + ~280 AWS calls |
| GitHub rate limit | 5,000 req/hr authenticated — no concern |
| AWS Cost Explorer | 10K requests/month limit — well within budget |
| LLM tokens per audit | ~50K tokens (summarization only) — ~$0.15/run |
| Compute | Single 2-vCPU / 4GB pod sufficient; P99 runtime <15 min |

---

## Results

| Metric | Outcome |
|---|---|
| Manual audit effort eliminated | ~2–3 engineer-weeks/quarter → 0 |
| Cloud waste identified | $120K+ in underutilized compute surfaced |
| Right-sizing actions taken | Multiple EC2 and RDS instances downsized |
| Repos below CI/CD maturity threshold | Identified and tracked to remediation |
| Leadership reporting cadence | Weekly automated digest vs. quarterly manual |

---

## Open Questions / Future Work

- [ ] Extend coverage_analyzer to support Go and Python repos (currently JVM-focused via JaCoCo)
- [ ] Add Kubernetes resource audit node (HPA tuning, pod request/limit drift)
- [ ] Build self-service remediation PRs: agent auto-opens PR to add missing CI caching config
- [ ] Integrate with Backstage service catalog for ownership enrichment
- [ ] Add cost forecasting: project 90-day spend trajectory based on utilization trends

---

## Decision Log

| Date | Decision | Rationale |
|---|---|---|
| 2025-Q1 | Use LangGraph over raw LLM calls | Native state management + parallel execution |
| 2025-Q1 | LLM used only for summarization, not metric collection | Determinism and cost control |
| 2025-Q2 | Add incremental/webhook-triggered mode | PR-level feedback loop without full audit cost |
| 2025-Q2 | Materialized view for dashboard | Avoid real-time query overhead on findings table |
