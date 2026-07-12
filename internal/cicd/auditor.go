// Package cicd scores CI/CD pipeline YAML against a 6-dimension maturity rubric.
package cicd

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/tazzledazzle/build-agent-smith/internal/domain"
	"gopkg.in/yaml.v3"
)

// Dimensions holds per-rubric scores. Totals 0–10.
type Dimensions struct {
	Caching         int // max 1
	TestGate        int // max 2
	Parallelism     int // max 1
	ArtifactPinning int // max 1
	DeployControls  int // max 2
	SecretHygiene   int // max 3
}

// Result is the scored output for one repository pipeline.
type Result struct {
	RepoName         string
	TotalScore       int
	Dimensions       Dimensions
	NeedsRemediation bool
	Findings         []domain.Finding
}

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(password|passwd|pwd|secret|api[_-]?key|token)\s*[=:]\s*['"]?[^\s'"]{8,}`),
	regexp.MustCompile(`(?i)AKIA[0-9A-Z]{16}`),
	regexp.MustCompile(`-----BEGIN (RSA |EC )?PRIVATE KEY-----`),
}

// ScorePipeline parses GitHub Actions or GitLab CI YAML and scores maturity.
func ScorePipeline(ctx context.Context, repoName, pipelineYAML string) (*Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("score pipeline: %w", err)
	}
	if strings.TrimSpace(pipelineYAML) == "" {
		return nil, fmt.Errorf("score pipeline %s: empty YAML", repoName)
	}

	var root map[string]any
	if err := yaml.Unmarshal([]byte(pipelineYAML), &root); err != nil {
		return nil, fmt.Errorf("score pipeline %s: parse YAML: %w", repoName, err)
	}

	dim := Dimensions{}
	var findings []domain.Finding

	dim.Caching = scoreCaching(root, pipelineYAML)
	dim.TestGate = scoreTestGate(root, pipelineYAML)
	dim.Parallelism = scoreParallelism(root)
	dim.ArtifactPinning = scoreArtifactPinning(pipelineYAML)
	dim.DeployControls = scoreDeployControls(root, pipelineYAML)
	dim.SecretHygiene = scoreSecretHygiene(pipelineYAML)

	if dim.Caching == 0 {
		findings = append(findings, gapFinding(repoName, domain.SeverityMedium,
			"Missing dependency caching",
			"Pipeline does not configure dependency caching", 2))
	}
	if dim.TestGate == 0 {
		findings = append(findings, gapFinding(repoName, domain.SeverityHigh,
			"No test gate before deploy",
			"Deploy jobs do not depend on a test job", 4))
	}
	if dim.ArtifactPinning == 0 {
		findings = append(findings, gapFinding(repoName, domain.SeverityMedium,
			"Unpinned image tags",
			"Pipeline uses :latest or unpinned tags", 1))
	}
	if dim.DeployControls == 0 {
		findings = append(findings, gapFinding(repoName, domain.SeverityHigh,
			"Missing production deploy controls",
			"No manual approval or environment protection for production", 3))
	}
	if dim.SecretHygiene == 0 {
		findings = append(findings, gapFinding(repoName, domain.SeverityCritical,
			"Hardcoded secrets detected",
			"Pipeline YAML contains patterns matching hardcoded credentials", 2))
	}

	total := dim.Caching + dim.TestGate + dim.Parallelism + dim.ArtifactPinning + dim.DeployControls + dim.SecretHygiene
	return &Result{
		RepoName:         repoName,
		TotalScore:       total,
		Dimensions:       dim,
		NeedsRemediation: total < 6,
		Findings:         findings,
	}, nil
}

func gapFinding(repo string, sev domain.Severity, title, desc string, hours float64) domain.Finding {
	f := domain.Finding{
		RepoName:               repo,
		FindingType:            domain.FindingTypeCICDGap,
		Severity:               sev,
		Title:                  title,
		Description:            desc,
		EstimatedCostUSD:       0,
		RemediationEffortHours: hours,
		Status:                 domain.FindingStatusOpen,
		CreatedAt:              time.Now().UTC(),
	}
	f.PriorityScore = domain.PriorityScore(f.EstimatedCostUSD, f.Severity, f.RemediationEffortHours)
	return f
}

func scoreCaching(root map[string]any, raw string) int {
	lower := strings.ToLower(raw)
	if strings.Contains(lower, "actions/cache") || strings.Contains(lower, "cache:") {
		return 1
	}
	// GitLab cache key at job or top level already covered by "cache:"
	_ = root
	return 0
}

func scoreTestGate(root map[string]any, raw string) int {
	hasTest := pipelineHasTests(raw)
	if !hasTest {
		return 0
	}

	// GitHub Actions: deploy-like job with needs referencing test
	if jobs, ok := root["jobs"].(map[string]any); ok {
		for name, jobVal := range jobs {
			job, ok := jobVal.(map[string]any)
			if !ok {
				continue
			}
			if !isDeployish(name, job) {
				continue
			}
			if needsContainsTest(job["needs"]) {
				return 2
			}
		}
		// If there is a test job and a separate deploy job with any needs, award
		hasTestJob := false
		hasDeployWithNeeds := false
		for name, jobVal := range jobs {
			job, _ := jobVal.(map[string]any)
			if strings.Contains(strings.ToLower(name), "test") {
				hasTestJob = true
			}
			if isDeployish(name, job) && job["needs"] != nil {
				hasDeployWithNeeds = true
			}
		}
		if hasTestJob && hasDeployWithNeeds {
			return 2
		}
		// Test job present but no deploy — still a test gate for CI
		if hasTestJob {
			return 2
		}
	}

	// GitLab: stages with test before deploy
	if stages, ok := root["stages"].([]any); ok {
		testIdx, deployIdx := -1, -1
		for i, s := range stages {
			name := strings.ToLower(fmt.Sprint(s))
			if strings.Contains(name, "test") {
				testIdx = i
			}
			if strings.Contains(name, "deploy") {
				deployIdx = i
			}
		}
		if testIdx >= 0 && (deployIdx < 0 || testIdx < deployIdx) {
			return 2
		}
	}

	if hasTest {
		return 2
	}
	return 0
}

// pipelineHasTests detects real test commands without matching "latest".
func pipelineHasTests(raw string) bool {
	lower := strings.ToLower(raw)
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`\bgo\s+test\b`),
		regexp.MustCompile(`\bnpm\s+test\b`),
		regexp.MustCompile(`\byarn\s+test\b`),
		regexp.MustCompile(`\bpytest\b`),
		regexp.MustCompile(`\bmvn\s+test\b`),
		regexp.MustCompile(`\bmake\s+test\b`),
		regexp.MustCompile(`\bcargo\s+test\b`),
		regexp.MustCompile(`\b(npm|yarn|pnpm)\s+run\s+test\b`),
		regexp.MustCompile(`(?m)^\s*test\s*:`),  // GitLab/GitHub job named test
		regexp.MustCompile(`(?m)^\s+test:\s*$`), // nested job key
		regexp.MustCompile(`\bstage:\s*test\b`),
		regexp.MustCompile(`jobs:\s*\n\s+test\s*:`),
	}
	for _, p := range patterns {
		if p.MatchString(lower) {
			return true
		}
	}
	return false
}

func isDeployish(name string, job map[string]any) bool {
	n := strings.ToLower(name)
	if strings.Contains(n, "deploy") || strings.Contains(n, "release") || strings.Contains(n, "prod") {
		return true
	}
	if env, ok := job["environment"]; ok {
		_ = env
		return true
	}
	return false
}

func needsContainsTest(needs any) bool {
	switch v := needs.(type) {
	case string:
		return strings.Contains(strings.ToLower(v), "test")
	case []any:
		for _, item := range v {
			if strings.Contains(strings.ToLower(fmt.Sprint(item)), "test") {
				return true
			}
		}
	}
	return false
}

func scoreParallelism(root map[string]any) int {
	if jobs, ok := root["jobs"].(map[string]any); ok && len(jobs) >= 2 {
		return 1
	}
	// GitLab: multiple jobs
	jobCount := 0
	for k, v := range root {
		if k == "stages" || k == "variables" || k == "include" || k == "default" || k == "workflow" {
			continue
		}
		if m, ok := v.(map[string]any); ok {
			if _, hasScript := m["script"]; hasScript {
				jobCount++
			}
			if _, hasStage := m["stage"]; hasStage {
				jobCount++
			}
		}
	}
	if jobCount >= 2 {
		return 1
	}
	return 0
}

func scoreArtifactPinning(raw string) int {
	lower := strings.ToLower(raw)
	if strings.Contains(lower, ":latest") {
		return 0
	}
	// Presence of versioned tags or digests indicates pinning practice
	pinned := regexp.MustCompile(`:[0-9]+\.[0-9]+|@sha256:[a-f0-9]{64}|actions/[a-z0-9-]+@v[0-9]`)
	if pinned.MatchString(lower) {
		return 1
	}
	// No latest and no obvious tags — treat as pinned (no anti-pattern)
	return 1
}

func scoreDeployControls(root map[string]any, raw string) int {
	lower := strings.ToLower(raw)
	if strings.Contains(lower, "when: manual") || strings.Contains(lower, "when:manual") {
		return 2
	}
	if jobs, ok := root["jobs"].(map[string]any); ok {
		for _, jobVal := range jobs {
			job, ok := jobVal.(map[string]any)
			if !ok {
				continue
			}
			if env, ok := job["environment"]; ok {
				switch e := env.(type) {
				case string:
					if strings.Contains(strings.ToLower(e), "prod") {
						return 2
					}
				case map[string]any:
					name := strings.ToLower(fmt.Sprint(e["name"]))
					if strings.Contains(name, "prod") {
						return 2
					}
				}
			}
		}
	}
	return 0
}

func scoreSecretHygiene(raw string) int {
	for _, pat := range secretPatterns {
		if pat.MatchString(raw) {
			return 0
		}
	}
	return 3
}
