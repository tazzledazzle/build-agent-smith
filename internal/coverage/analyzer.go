// Package coverage analyzes test coverage reports and change-risk correlation.
package coverage

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"math"
	"time"

	"github.com/tazzledazzle/build-agent-smith/internal/domain"
)

const lineThreshold = 60.0

// Report is a per-repo coverage vector input.
type Report struct {
	RepoName       string
	LinePct        float64
	BranchPct      float64
	CommitsPerWeek float64
}

// Result is the analyzed coverage output.
type Result struct {
	RepoName        string
	LinePct         float64
	BranchPct       float64
	ChangeRiskScore float64
	Findings        []domain.Finding
}

// Analyze correlates coverage with change frequency and flags repos below threshold.
func Analyze(ctx context.Context, report Report) (*Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("analyze coverage: %w", err)
	}

	risk := changeRisk(report.LinePct, report.CommitsPerWeek)
	result := &Result{
		RepoName:        report.RepoName,
		LinePct:         report.LinePct,
		BranchPct:       report.BranchPct,
		ChangeRiskScore: risk,
	}

	if report.LinePct < lineThreshold {
		sev := domain.SeverityMedium
		if risk >= 0.5 || report.LinePct < 40 || report.CommitsPerWeek >= 10 {
			sev = domain.SeverityHigh
		}
		f := domain.Finding{
			RepoName:               report.RepoName,
			FindingType:            domain.FindingTypeCoverageRisk,
			Severity:               sev,
			Title:                  "Line coverage below threshold",
			Description:            fmt.Sprintf("Line coverage %.1f%% is below %.0f%% threshold (change risk %.2f)", report.LinePct, lineThreshold, risk),
			RemediationEffortHours: 8,
			Status:                 domain.FindingStatusOpen,
			CreatedAt:              time.Now().UTC(),
		}
		f.PriorityScore = domain.PriorityScore(f.EstimatedCostUSD, f.Severity, f.RemediationEffortHours)
		result.Findings = append(result.Findings, f)
	}

	return result, nil
}

// changeRisk = (threshold - line_pct)/threshold * log1p(commits_per_week)
// High churn + low coverage → elevated risk. Healthy coverage → 0.
func changeRisk(linePct, commitsPerWeek float64) float64 {
	if linePct >= lineThreshold {
		return 0
	}
	gap := (lineThreshold - linePct) / lineThreshold
	churn := math.Log1p(math.Max(0, commitsPerWeek))
	return math.Round(gap*churn*100) / 100
}

type jacocoReport struct {
	Counters []jacocoCounter `xml:"counter"`
}

type jacocoCounter struct {
	Type    string `xml:"type,attr"`
	Missed  int    `xml:"missed,attr"`
	Covered int    `xml:"covered,attr"`
}

// ParseJaCoCoXML extracts line/branch percentages from a JaCoCo XML report.
func ParseJaCoCoXML(repoName, raw string, commitsPerWeek float64) (*Report, error) {
	var doc jacocoReport
	if err := xml.Unmarshal([]byte(raw), &doc); err != nil {
		return nil, fmt.Errorf("parse jacoco: %w", err)
	}
	report := &Report{RepoName: repoName, CommitsPerWeek: commitsPerWeek}
	for _, c := range doc.Counters {
		total := float64(c.Missed + c.Covered)
		if total == 0 {
			continue
		}
		pct := float64(c.Covered) / total * 100
		switch c.Type {
		case "LINE":
			report.LinePct = pct
		case "BRANCH":
			report.BranchPct = pct
		}
	}
	return report, nil
}

type jsonCoverage struct {
	LinePct   float64 `json:"line_pct"`
	BranchPct float64 `json:"branch_pct"`
}

// ParseJSON extracts coverage percentages from Codecov-style JSON.
func ParseJSON(repoName, raw string, commitsPerWeek float64) (*Report, error) {
	var doc jsonCoverage
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return nil, fmt.Errorf("parse coverage json: %w", err)
	}
	return &Report{
		RepoName:       repoName,
		LinePct:        doc.LinePct,
		BranchPct:      doc.BranchPct,
		CommitsPerWeek: commitsPerWeek,
	}, nil
}
