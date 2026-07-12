// Package aggregator merges worker outputs, deduplicates, scores DORA, and ranks findings.
package aggregator

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/tazzledazzle/build-agent-smith/internal/domain"
)

// RepoMetrics feeds DORA composite and repo_scores rows.
type RepoMetrics struct {
	CICDScore            int
	StandardizationScore int
	CoverageLinePct      float64
	CoverageRisk         float64
	DeploysPerWeek       float64
	LeadTimeDays         float64
	ChangeFailureRate    float64
	MTTRHours            float64
}

// Input is the merged worker payload for aggregation.
type Input struct {
	Findings    []domain.Finding
	RepoMetrics map[string]RepoMetrics
}

// Output is the ranked, deduplicated aggregation result.
type Output struct {
	Findings         []domain.Finding
	ExecutiveSummary []domain.Finding
	DoraScores       map[string]domain.DoraScore
	RepoScores       map[string]domain.RepoScore
}

// Aggregate deduplicates findings, applies DORA scoring, and ranks by priority.
func Aggregate(ctx context.Context, in Input) (*Output, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("aggregate: %w", err)
	}

	deduped := dedupe(in.Findings)
	for i := range deduped {
		deduped[i].PriorityScore = domain.PriorityScore(
			deduped[i].EstimatedCostUSD,
			deduped[i].Severity,
			deduped[i].RemediationEffortHours,
		)
	}
	sort.SliceStable(deduped, func(i, j int) bool {
		return deduped[i].PriorityScore > deduped[j].PriorityScore
	})

	summary := deduped
	if len(summary) > 10 {
		summary = summary[:10]
	}

	dora := make(map[string]domain.DoraScore, len(in.RepoMetrics))
	repoScores := make(map[string]domain.RepoScore, len(in.RepoMetrics))
	for name, m := range in.RepoMetrics {
		ds := computeDORA(m)
		dora[name] = ds
		repoScores[name] = domain.RepoScore{
			RepoName:             name,
			StandardizationScore: m.StandardizationScore,
			CICDMaturityScore:    m.CICDScore,
			CoverageLinePct:      m.CoverageLinePct,
			CoverageRiskScore:    m.CoverageRisk,
			DoraCompositeScore:   ds.Composite,
		}
	}

	return &Output{
		Findings:         deduped,
		ExecutiveSummary: append([]domain.Finding(nil), summary...),
		DoraScores:       dora,
		RepoScores:       repoScores,
	}, nil
}

func dedupe(findings []domain.Finding) []domain.Finding {
	seen := make(map[string]struct{}, len(findings))
	out := make([]domain.Finding, 0, len(findings))
	for _, f := range findings {
		key := strings.Join([]string{f.RepoName, string(f.FindingType), f.Title, f.ResourceID}, "|")
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, f)
	}
	return out
}

// computeDORA maps delivery proxies into a 0–10 composite score.
func computeDORA(m RepoMetrics) domain.DoraScore {
	freq := clamp(m.DeploysPerWeek/5.0*10, 0, 10)         // 5+/week → 10
	lead := clamp((14-m.LeadTimeDays)/14*10, 0, 10)       // ≤0 days → 10, ≥14 → 0
	cfr := clamp((0.5-m.ChangeFailureRate)/0.5*10, 0, 10) // 0% → 10, ≥50% → 0
	mttr := clamp((48-m.MTTRHours)/48*10, 0, 10)          // 0h → 10, ≥48h → 0
	composite := (freq + lead + cfr + mttr) / 4
	return domain.DoraScore{
		DeploymentFrequency: m.DeploysPerWeek,
		LeadTimeDays:        m.LeadTimeDays,
		ChangeFailureRate:   m.ChangeFailureRate,
		MTTRHours:           m.MTTRHours,
		Composite:           round2(composite),
	}
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}
