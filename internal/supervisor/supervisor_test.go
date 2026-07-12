package supervisor_test

import (
	"context"
	"testing"

	"github.com/tazzledazzle/build-agent-smith/internal/domain"
	"github.com/tazzledazzle/build-agent-smith/internal/supervisor"
)

func TestPlan_FullScopeInvokesAllWorkers(t *testing.T) {
	plan, err := supervisor.Plan(context.Background(), supervisor.Request{
		Scope: domain.ScopeFull,
		Repos: []domain.RepoConfig{{Name: "a"}, {Name: "b"}},
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	want := []string{"repo_scanner", "cicd_auditor", "coverage_analyzer", "cloud_finops"}
	if !equalStrings(plan.Workers, want) {
		t.Errorf("Workers = %v, want %v", plan.Workers, want)
	}
	if len(plan.Repos) != 2 {
		t.Errorf("Repos = %d, want 2", len(plan.Repos))
	}
	if plan.AuditRunID == "" {
		t.Error("AuditRunID should be set")
	}
}

func TestPlan_FinOpsOnly(t *testing.T) {
	plan, err := supervisor.Plan(context.Background(), supervisor.Request{
		Scope: domain.ScopeFinOpsOnly,
		Repos: []domain.RepoConfig{{Name: "a"}},
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if !equalStrings(plan.Workers, []string{"cloud_finops"}) {
		t.Errorf("Workers = %v, want [cloud_finops]", plan.Workers)
	}
}

func TestPlan_IncrementalSingleRepo(t *testing.T) {
	plan, err := supervisor.Plan(context.Background(), supervisor.Request{
		Scope:          domain.ScopeIncremental,
		Repos:          []domain.RepoConfig{{Name: "a"}, {Name: "b"}},
		TargetRepoName: "b",
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	want := []string{"repo_scanner", "cicd_auditor", "coverage_analyzer"}
	if !equalStrings(plan.Workers, want) {
		t.Errorf("Workers = %v, want %v", plan.Workers, want)
	}
	if len(plan.Repos) != 1 || plan.Repos[0].Name != "b" {
		t.Errorf("Repos = %+v, want single repo b", plan.Repos)
	}
}

func TestPlan_IncrementalMissingTarget(t *testing.T) {
	_, err := supervisor.Plan(context.Background(), supervisor.Request{
		Scope:          domain.ScopeIncremental,
		Repos:          []domain.RepoConfig{{Name: "a"}},
		TargetRepoName: "missing",
	})
	if err == nil {
		t.Fatal("expected error for missing target repo")
	}
}

func TestPlan_EmptyRepos(t *testing.T) {
	_, err := supervisor.Plan(context.Background(), supervisor.Request{
		Scope: domain.ScopeFull,
	})
	if err == nil {
		t.Fatal("expected error for empty repos")
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
