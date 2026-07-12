package aggregator_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/tazzledazzle/build-agent-smith/internal/aggregator"
	"github.com/tazzledazzle/build-agent-smith/internal/domain"
)

func TestAggregate_DeduplicatesIdenticalFindings(t *testing.T) {
	input := []domain.Finding{
		{RepoName: "a", FindingType: domain.FindingTypeCICDGap, Title: "Missing cache", Severity: domain.SeverityMedium, EstimatedCostUSD: 0, RemediationEffortHours: 2},
		{RepoName: "a", FindingType: domain.FindingTypeCICDGap, Title: "Missing cache", Severity: domain.SeverityMedium, EstimatedCostUSD: 0, RemediationEffortHours: 2},
		{RepoName: "a", FindingType: domain.FindingTypeCoverageRisk, Title: "Low coverage", Severity: domain.SeverityHigh, EstimatedCostUSD: 0, RemediationEffortHours: 8},
	}
	out, err := aggregator.Aggregate(context.Background(), aggregator.Input{
		Findings: input,
		RepoMetrics: map[string]aggregator.RepoMetrics{
			"a": {CICDScore: 4, StandardizationScore: 2, CoverageLinePct: 45, CoverageRisk: 3, DeploysPerWeek: 1, LeadTimeDays: 5, ChangeFailureRate: 0.2, MTTRHours: 8},
		},
	})
	if err != nil {
		t.Fatalf("Aggregate: %v", err)
	}
	if len(out.Findings) != 2 {
		t.Fatalf("Findings = %d, want 2 after dedupe", len(out.Findings))
	}
}

func TestAggregate_RanksByPriority(t *testing.T) {
	input := []domain.Finding{
		{RepoName: "a", Title: "low", Severity: domain.SeverityLow, EstimatedCostUSD: 10, RemediationEffortHours: 10},
		{RepoName: "a", Title: "critical waste", Severity: domain.SeverityCritical, EstimatedCostUSD: 1000, RemediationEffortHours: 2},
		{RepoName: "a", Title: "medium", Severity: domain.SeverityMedium, EstimatedCostUSD: 100, RemediationEffortHours: 5},
	}
	out, err := aggregator.Aggregate(context.Background(), aggregator.Input{Findings: input})
	if err != nil {
		t.Fatalf("Aggregate: %v", err)
	}
	if out.Findings[0].Title != "critical waste" {
		t.Errorf("top finding = %q, want critical waste", out.Findings[0].Title)
	}
	if out.Findings[0].PriorityScore <= out.Findings[1].PriorityScore {
		t.Error("findings not sorted by descending priority")
	}
}

func TestAggregate_ComputesDORAScores(t *testing.T) {
	out, err := aggregator.Aggregate(context.Background(), aggregator.Input{
		RepoMetrics: map[string]aggregator.RepoMetrics{
			"fast": {DeploysPerWeek: 10, LeadTimeDays: 0.5, ChangeFailureRate: 0.05, MTTRHours: 1, CICDScore: 9, StandardizationScore: 5, CoverageLinePct: 90},
			"slow": {DeploysPerWeek: 0.2, LeadTimeDays: 14, ChangeFailureRate: 0.4, MTTRHours: 48, CICDScore: 2, StandardizationScore: 1, CoverageLinePct: 40},
		},
	})
	if err != nil {
		t.Fatalf("Aggregate: %v", err)
	}
	if out.DoraScores["fast"].Composite <= out.DoraScores["slow"].Composite {
		t.Errorf("fast composite %v should exceed slow %v",
			out.DoraScores["fast"].Composite, out.DoraScores["slow"].Composite)
	}
}

func TestAggregate_ExecutiveSummaryTop10(t *testing.T) {
	var findings []domain.Finding
	for i := 0; i < 15; i++ {
		findings = append(findings, domain.Finding{
			RepoName: "r", Title: fmt.Sprintf("finding-%d", i), Severity: domain.SeverityHigh,
			EstimatedCostUSD: float64(100 - i), RemediationEffortHours: 1,
		})
	}
	out, err := aggregator.Aggregate(context.Background(), aggregator.Input{Findings: findings})
	if err != nil {
		t.Fatalf("Aggregate: %v", err)
	}
	if len(out.ExecutiveSummary) != 10 {
		t.Errorf("ExecutiveSummary len = %d, want 10", len(out.ExecutiveSummary))
	}
	if len(out.Findings) != 15 {
		t.Errorf("full Findings len = %d, want 15", len(out.Findings))
	}
}
