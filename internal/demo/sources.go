// Package demo provides deterministic in-process data sources for local demos.
package demo

import (
	"context"

	"github.com/tazzledazzle/build-agent-smith/internal/agent"
	"github.com/tazzledazzle/build-agent-smith/internal/finops"
)

// Sources implements agent.Dependencies with fixture data.
type Sources struct{}

// HasFile reports a realistic mix of standardization artifacts.
func (Sources) HasFile(_ context.Context, _, repo, path string) (bool, error) {
	switch repo {
	case "payments-api", "identity-svc":
		return true, nil
	case "platform-infra":
		return path == "Dockerfile" || path == ".gitlab-ci.yml" || path == ".github/workflows/ci.yml", nil
	default:
		return false, nil
	}
}

// FetchPipelineYAML returns sample CI configs per repo.
func (Sources) FetchPipelineYAML(_ context.Context, _, repo string) (string, error) {
	switch repo {
	case "payments-api", "identity-svc":
		return `
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
      - run: deploy --tag v1.2.3
`, nil
	default:
		return `
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - run: docker push app:latest
`, nil
	}
}

// FetchCoverage returns sample coverage vectors.
func (Sources) FetchCoverage(_ context.Context, repo string) (agent.CoverageInput, error) {
	switch repo {
	case "payments-api":
		return agent.CoverageInput{LinePct: 82, BranchPct: 70, CommitsPerWeek: 4}, nil
	case "identity-svc":
		return agent.CoverageInput{LinePct: 65, BranchPct: 50, CommitsPerWeek: 6}, nil
	default:
		return agent.CoverageInput{LinePct: 35, BranchPct: 20, CommitsPerWeek: 12}, nil
	}
}

// FetchCloudInventory returns sample AWS waste signals.
func (Sources) FetchCloudInventory(_ context.Context) (finops.Inventory, error) {
	return finops.Inventory{
		EC2Instances: []finops.EC2Instance{
			{ID: "i-idle01", AvgCPU14d: 2.1, EstimatedMonthlyUSD: 180, Tags: map[string]string{"team": "platform", "service": "batch"}},
			{ID: "i-ok01", AvgCPU14d: 42, EstimatedMonthlyUSD: 90, Tags: map[string]string{"team": "payments", "service": "payments-api"}},
			{ID: "i-naked", AvgCPU14d: 30, EstimatedMonthlyUSD: 70, Tags: map[string]string{}},
		},
		RDSInstances: []finops.RDSInstance{
			{ID: "db-fat", AvgCPUUtilization: 8, EstimatedMonthlyUSD: 450, Tags: map[string]string{"team": "data", "service": "analytics"}},
		},
		EBSVolumes: []finops.EBSVolume{
			{ID: "vol-orphan", Attached: false, EstimatedMonthlyUSD: 35, Tags: map[string]string{"team": "infra", "service": "logs"}},
		},
	}, nil
}
