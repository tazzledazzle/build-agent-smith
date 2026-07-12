// Package reposcanner scores repository standardization against org templates.
package reposcanner

import (
	"context"
	"fmt"
	"time"

	"github.com/tazzledazzle/build-agent-smith/internal/domain"
)

// FileChecker checks whether a path exists in a remote repository.
type FileChecker interface {
	HasFile(ctx context.Context, owner, repo, path string) (bool, error)
}

// Result is the standardization scan output for one repo.
type Result struct {
	RepoName             string
	StandardizationScore int // 0–5
	Present              map[string]bool
	Findings             []domain.Finding
}

// Scanner performs standardization checks via a FileChecker.
type Scanner struct {
	files FileChecker
}

// New creates a Scanner.
func New(files FileChecker) *Scanner {
	return &Scanner{files: files}
}

// dimensionPaths maps rubric dimensions to candidate file paths (any match scores).
var dimensionPaths = []struct {
	name  string
	paths []string
}{
	{"ci_config", []string{".github/workflows/ci.yml", ".github/workflows/ci.yaml", ".gitlab-ci.yml"}},
	{"dockerfile", []string{"Dockerfile"}},
	{"pre_commit", []string{".pre-commit-config.yaml"}},
	{"codeowners", []string{"CODEOWNERS", ".github/CODEOWNERS", "docs/CODEOWNERS"}},
	{"lint_config", []string{".golangci.yml", ".golangci.yaml", ".eslintrc.json", ".eslintrc.yml", "eslint.config.js", "checkstyle.xml"}},
}

// Scan evaluates standardization for a single repository (0–5 rubric).
func (s *Scanner) Scan(ctx context.Context, repo domain.RepoConfig) (*Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("scan %s: %w", repo.Name, err)
	}

	present := make(map[string]bool, len(dimensionPaths))
	score := 0
	var findings []domain.Finding

	for _, dim := range dimensionPaths {
		found := false
		for _, path := range dim.paths {
			ok, err := s.files.HasFile(ctx, repo.Owner, repo.Name, path)
			if err != nil {
				return nil, fmt.Errorf("scan %s path %s: %w", repo.Name, path, err)
			}
			if ok {
				found = true
				present[path] = true
				break
			}
		}
		present[dim.name] = found
		if found {
			score++
			continue
		}
		findings = append(findings, domain.Finding{
			RepoName:               repo.Name,
			FindingType:            domain.FindingTypeStandardization,
			Severity:               domain.SeverityMedium,
			Title:                  fmt.Sprintf("Missing %s", dim.name),
			Description:            fmt.Sprintf("Repository %s lacks required %s artifact", repo.Name, dim.name),
			RemediationEffortHours: 2,
			Status:                 domain.FindingStatusOpen,
			CreatedAt:              time.Now().UTC(),
		})
		findings[len(findings)-1].PriorityScore = domain.PriorityScore(0, domain.SeverityMedium, 2)
	}

	return &Result{
		RepoName:             repo.Name,
		StandardizationScore: score,
		Present:              present,
		Findings:             findings,
	}, nil
}
