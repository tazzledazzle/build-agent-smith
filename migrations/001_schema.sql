-- Platform Maturity Audit Agent schema (from TDD.md)

CREATE TABLE IF NOT EXISTS audit_runs (
    id UUID PRIMARY KEY,
    triggered_at TIMESTAMPTZ,
    scope TEXT,
    repo_count INT,
    total_findings INT,
    total_estimated_waste_usd NUMERIC(10,2),
    completed_at TIMESTAMPTZ,
    status TEXT
);

CREATE TABLE IF NOT EXISTS repo_scores (
    audit_run_id UUID REFERENCES audit_runs(id),
    repo_name TEXT,
    standardization_score INT,
    cicd_maturity_score INT,
    coverage_line_pct NUMERIC(5,2),
    coverage_risk_score NUMERIC(5,2),
    dora_composite_score NUMERIC(5,2),
    PRIMARY KEY (audit_run_id, repo_name)
);

CREATE TABLE IF NOT EXISTS findings (
    id UUID PRIMARY KEY,
    audit_run_id UUID REFERENCES audit_runs(id),
    repo_name TEXT,
    finding_type TEXT,
    severity TEXT,
    title TEXT,
    description TEXT,
    estimated_cost_usd NUMERIC(10,2),
    remediation_effort_hours INT,
    priority_score NUMERIC(10,4),
    resource_id TEXT,
    owner_tag TEXT,
    status TEXT DEFAULT 'open',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE MATERIALIZED VIEW IF NOT EXISTS platform_health_current AS
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
WHERE a.id = (SELECT id FROM audit_runs ORDER BY completed_at DESC NULLS LAST LIMIT 1)
GROUP BY r.repo_name, r.cicd_maturity_score, r.coverage_line_pct, r.dora_composite_score;
