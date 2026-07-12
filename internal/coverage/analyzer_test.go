package coverage_test

import (
	"context"
	"testing"

	"github.com/tazzledazzle/build-agent-smith/internal/coverage"
	"github.com/tazzledazzle/build-agent-smith/internal/domain"
)

func TestAnalyze_BelowThreshold(t *testing.T) {
	report := coverage.Report{
		RepoName:       "risk-svc",
		LinePct:        42.5,
		BranchPct:      30.0,
		CommitsPerWeek: 12,
	}
	result, err := coverage.Analyze(context.Background(), report)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.LinePct != 42.5 {
		t.Errorf("LinePct = %v, want 42.5", result.LinePct)
	}
	if result.ChangeRiskScore <= 0 {
		t.Error("ChangeRiskScore should be > 0 for high-churn low-coverage")
	}
	if len(result.Findings) == 0 {
		t.Fatal("expected coverage_risk finding below 60%")
	}
	if result.Findings[0].FindingType != domain.FindingTypeCoverageRisk {
		t.Errorf("FindingType = %q", result.Findings[0].FindingType)
	}
	if result.Findings[0].Severity != domain.SeverityHigh {
		t.Errorf("Severity = %q, want high", result.Findings[0].Severity)
	}
}

func TestAnalyze_HealthyCoverage(t *testing.T) {
	report := coverage.Report{
		RepoName:       "healthy",
		LinePct:        85.0,
		BranchPct:      70.0,
		CommitsPerWeek: 3,
	}
	result, err := coverage.Analyze(context.Background(), report)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.ChangeRiskScore != 0 {
		t.Errorf("ChangeRiskScore = %v, want 0", result.ChangeRiskScore)
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected no findings, got %d", len(result.Findings))
	}
}

func TestAnalyze_HighChurnRaisesRisk(t *testing.T) {
	lowChurn, err := coverage.Analyze(context.Background(), coverage.Report{
		RepoName: "a", LinePct: 50, BranchPct: 40, CommitsPerWeek: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	highChurn, err := coverage.Analyze(context.Background(), coverage.Report{
		RepoName: "b", LinePct: 50, BranchPct: 40, CommitsPerWeek: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if highChurn.ChangeRiskScore <= lowChurn.ChangeRiskScore {
		t.Errorf("high churn risk %v should exceed low churn risk %v",
			highChurn.ChangeRiskScore, lowChurn.ChangeRiskScore)
	}
}

func TestParseJaCoCoXML(t *testing.T) {
	xml := `<?xml version="1.0"?>
<report name="demo">
  <counter type="LINE" missed="40" covered="60"/>
  <counter type="BRANCH" missed="20" covered="30"/>
</report>`
	report, err := coverage.ParseJaCoCoXML("demo", xml, 5)
	if err != nil {
		t.Fatalf("ParseJaCoCoXML: %v", err)
	}
	if report.LinePct != 60.0 {
		t.Errorf("LinePct = %v, want 60", report.LinePct)
	}
	if report.BranchPct != 60.0 {
		t.Errorf("BranchPct = %v, want 60", report.BranchPct)
	}
}

func TestParseCoverageJSON(t *testing.T) {
	raw := `{"line_pct": 72.5, "branch_pct": 55.0}`
	report, err := coverage.ParseJSON("svc", raw, 2)
	if err != nil {
		t.Fatalf("ParseJSON: %v", err)
	}
	if report.LinePct != 72.5 {
		t.Errorf("LinePct = %v, want 72.5", report.LinePct)
	}
}
