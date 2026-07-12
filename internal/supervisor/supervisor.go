// Package supervisor plans audit scope and routes work to worker nodes.
package supervisor

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/tazzledazzle/build-agent-smith/internal/domain"
)

// Request is the input to plan an audit cycle.
type Request struct {
	Scope          domain.Scope
	Repos          []domain.RepoConfig
	TargetRepoName string // required for incremental
}

// PlanResult describes which workers run against which repos.
type PlanResult struct {
	AuditRunID string
	Scope      domain.Scope
	Repos      []domain.RepoConfig
	Workers    []string
}

// Plan partitions work and selects workers based on audit scope.
func Plan(ctx context.Context, req Request) (*PlanResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("plan: %w", err)
	}
	if len(req.Repos) == 0 {
		return nil, fmt.Errorf("plan: repository manifest is empty")
	}

	repos := req.Repos
	var workers []string

	switch req.Scope {
	case domain.ScopeFull:
		workers = []string{"repo_scanner", "cicd_auditor", "coverage_analyzer", "cloud_finops"}
	case domain.ScopeFinOpsOnly:
		workers = []string{"cloud_finops"}
	case domain.ScopeIncremental:
		if req.TargetRepoName == "" {
			return nil, fmt.Errorf("plan: incremental scope requires target repo")
		}
		found := false
		for _, r := range req.Repos {
			if r.Name == req.TargetRepoName {
				repos = []domain.RepoConfig{r}
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("plan: target repo %q not in manifest", req.TargetRepoName)
		}
		workers = []string{"repo_scanner", "cicd_auditor", "coverage_analyzer"}
	default:
		return nil, fmt.Errorf("plan: unknown scope %q", req.Scope)
	}

	return &PlanResult{
		AuditRunID: uuid.NewString(),
		Scope:      req.Scope,
		Repos:      repos,
		Workers:    workers,
	}, nil
}
