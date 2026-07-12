package agent_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tazzledazzle/build-agent-smith/internal/agent"
	"github.com/tazzledazzle/build-agent-smith/internal/domain"
	"github.com/tazzledazzle/build-agent-smith/internal/finops"
)

type stubDeps struct {
	files map[string]map[string]bool // repo -> path -> present
	yamls map[string]string
	cov   map[string]agent.CoverageInput
	cloud finops.Inventory
}

func (s *stubDeps) HasFile(_ context.Context, _, repo, path string) (bool, error) {
	if m, ok := s.files[repo]; ok {
		return m[path], nil
	}
	return false, nil
}

func (s *stubDeps) FetchPipelineYAML(_ context.Context, _, repo string) (string, error) {
	return s.yamls[repo], nil
}

func (s *stubDeps) FetchCoverage(_ context.Context, repo string) (agent.CoverageInput, error) {
	return s.cov[repo], nil
}

func (s *stubDeps) FetchCloudInventory(_ context.Context) (finops.Inventory, error) {
	return s.cloud, nil
}

func TestAgent_FullAuditProducesFindingsAndScores(t *testing.T) {
	deps := &stubDeps{
		files: map[string]map[string]bool{
			"svc-a": {
				".github/workflows/ci.yml": true,
				"Dockerfile":               true,
				".pre-commit-config.yaml":  true,
				"CODEOWNERS":               true,
				".golangci.yml":            true,
			},
		},
		yamls: map[string]string{
			"svc-a": `
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/cache@v4
      - run: go test ./...
  deploy:
    needs: test
    environment: {name: production}
    runs-on: ubuntu-latest
    steps:
      - run: deploy --tag v1.0.0
`,
		},
		cov: map[string]agent.CoverageInput{
			"svc-a": {LinePct: 55, BranchPct: 40, CommitsPerWeek: 8},
		},
		cloud: finops.Inventory{
			EC2Instances: []finops.EC2Instance{
				{ID: "i-1", AvgCPU14d: 2, EstimatedMonthlyUSD: 200, Tags: map[string]string{"team": "plat", "service": "svc-a"}},
			},
		},
	}

	a := agent.New(deps)
	state, err := a.Run(context.Background(), agent.RunRequest{
		Scope: domain.ScopeFull,
		Repos: []domain.RepoConfig{{Name: "svc-a", Owner: "acme", Provider: "github"}},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if state.AuditRunID == "" {
		t.Fatal("expected audit run id")
	}
	if len(state.Findings) == 0 {
		t.Fatal("expected findings from coverage and/or finops")
	}
	if _, ok := state.RepoScores["svc-a"]; !ok {
		t.Fatal("expected repo score for svc-a")
	}
	if state.Status != "COMPLETE" && state.Status != "PARTIAL_AUDIT" {
		t.Errorf("Status = %q", state.Status)
	}
}

func TestAgent_FinOpsOnlySkipsRepoWorkers(t *testing.T) {
	var fileCalls atomic.Int64
	deps := &countingDeps{stubDeps: stubDeps{
		cloud: finops.Inventory{
			EBSVolumes: []finops.EBSVolume{{ID: "vol-1", Attached: false, EstimatedMonthlyUSD: 30, Tags: map[string]string{"team": "a", "service": "b"}}},
		},
	}, fileCalls: &fileCalls}

	a := agent.New(deps)
	state, err := a.Run(context.Background(), agent.RunRequest{
		Scope: domain.ScopeFinOpsOnly,
		Repos: []domain.RepoConfig{{Name: "svc-a", Owner: "acme"}},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if fileCalls.Load() != 0 {
		t.Errorf("repo file checks = %d, want 0 for finops_only", fileCalls.Load())
	}
	if len(state.Findings) == 0 {
		t.Fatal("expected cloud waste findings")
	}
}

func TestAgent_RespectsCancellation(t *testing.T) {
	deps := &stubDeps{
		yamls: map[string]string{"svc-a": "jobs:\n  build:\n    steps:\n      - run: echo hi\n"},
		files: map[string]map[string]bool{"svc-a": {}},
		cov:   map[string]agent.CoverageInput{"svc-a": {LinePct: 80}},
	}
	a := agent.New(deps)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := a.Run(ctx, agent.RunRequest{
		Scope:          domain.ScopeIncremental,
		Repos:          []domain.RepoConfig{{Name: "svc-a", Owner: "acme"}},
		TargetRepoName: "svc-a",
	})
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
}

func TestAgent_ParallelExecutionCompletes(t *testing.T) {
	deps := &stubDeps{
		files: map[string]map[string]bool{
			"r1": {".github/workflows/ci.yml": true, "Dockerfile": true, ".pre-commit-config.yaml": true, "CODEOWNERS": true, ".golangci.yml": true},
			"r2": {},
		},
		yamls: map[string]string{
			"r1": "jobs:\n  test:\n    steps:\n      - run: go test ./...\n",
			"r2": "jobs:\n  deploy:\n    steps:\n      - run: echo password=supersecret123\n      - run: docker push x:latest\n",
		},
		cov: map[string]agent.CoverageInput{
			"r1": {LinePct: 90, CommitsPerWeek: 1},
			"r2": {LinePct: 20, CommitsPerWeek: 15},
		},
		cloud: finops.Inventory{},
	}
	a := agent.New(deps)
	start := time.Now()
	state, err := a.Run(context.Background(), agent.RunRequest{
		Scope: domain.ScopeFull,
		Repos: []domain.RepoConfig{
			{Name: "r1", Owner: "acme"},
			{Name: "r2", Owner: "acme"},
		},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if time.Since(start) > 5*time.Second {
		t.Error("full audit of 2 repos took too long")
	}
	if len(state.RepoScores) != 2 {
		t.Errorf("RepoScores = %d, want 2", len(state.RepoScores))
	}
}

type countingDeps struct {
	stubDeps
	fileCalls *atomic.Int64
}

func (c *countingDeps) HasFile(ctx context.Context, owner, repo, path string) (bool, error) {
	c.fileCalls.Add(1)
	return c.stubDeps.HasFile(ctx, owner, repo, path)
}
