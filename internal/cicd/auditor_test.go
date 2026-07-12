package cicd_test

import (
	"context"
	"testing"

	"github.com/tazzledazzle/build-agent-smith/internal/cicd"
	"github.com/tazzledazzle/build-agent-smith/internal/domain"
)

func TestScorePipeline_MatureGitHubActions(t *testing.T) {
	yaml := `
name: CI
on: [push]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/cache@v4
        with:
          path: ~/.cache
          key: deps
      - run: go test ./...
  deploy:
    needs: test
    environment:
      name: production
    runs-on: ubuntu-latest
    steps:
      - uses: docker/build-push-action@v5
        with:
          tags: myapp:1.2.3
`
	result, err := cicd.ScorePipeline(context.Background(), "payments-api", yaml)
	if err != nil {
		t.Fatalf("ScorePipeline: %v", err)
	}
	if result.RepoName != "payments-api" {
		t.Errorf("RepoName = %q, want payments-api", result.RepoName)
	}
	if result.TotalScore != 10 {
		t.Errorf("TotalScore = %d, want 10 (got dimensions: %+v)", result.TotalScore, result.Dimensions)
	}
	if result.NeedsRemediation {
		t.Error("NeedsRemediation = true, want false for score >= 6")
	}
}

func TestScorePipeline_ImmaturePipeline(t *testing.T) {
	yaml := `
name: Deploy
on: [push]
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - run: docker push myapp:latest
      - run: echo "password=supersecret123"
`
	result, err := cicd.ScorePipeline(context.Background(), "legacy-svc", yaml)
	if err != nil {
		t.Fatalf("ScorePipeline: %v", err)
	}
	// No cache (0), no test gate (0), no parallelism (0), latest tag (0 pin),
	// no deploy controls (0), secret hygiene fail (0) => 0
	if result.TotalScore != 0 {
		t.Errorf("TotalScore = %d, want 0", result.TotalScore)
	}
	if !result.NeedsRemediation {
		t.Error("NeedsRemediation = false, want true for score < 6")
	}
	if len(result.Findings) == 0 {
		t.Fatal("expected findings for immature pipeline")
	}
	foundSecret := false
	for _, f := range result.Findings {
		if f.FindingType == domain.FindingTypeCICDGap && f.Severity == domain.SeverityCritical {
			foundSecret = true
		}
	}
	if !foundSecret {
		t.Error("expected critical finding for hardcoded secret")
	}
}

func TestScorePipeline_PartialScores(t *testing.T) {
	tests := []struct {
		name         string
		yaml         string
		wantCache    int
		wantTestGate int
		wantParallel int
	}{
		{
			name: "caching only",
			yaml: `
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/cache@v3
      - run: make build
`,
			wantCache:    1,
			wantTestGate: 0,
			wantParallel: 0,
		},
		{
			name: "test before deploy",
			yaml: `
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - run: npm test
  deploy:
    needs: test
    runs-on: ubuntu-latest
    steps:
      - run: ./deploy.sh
`,
			wantCache:    0,
			wantTestGate: 2,
			wantParallel: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := cicd.ScorePipeline(context.Background(), "svc", tt.yaml)
			if err != nil {
				t.Fatalf("ScorePipeline: %v", err)
			}
			if result.Dimensions.Caching != tt.wantCache {
				t.Errorf("Caching = %d, want %d", result.Dimensions.Caching, tt.wantCache)
			}
			if result.Dimensions.TestGate != tt.wantTestGate {
				t.Errorf("TestGate = %d, want %d", result.Dimensions.TestGate, tt.wantTestGate)
			}
			if result.Dimensions.Parallelism != tt.wantParallel {
				t.Errorf("Parallelism = %d, want %d", result.Dimensions.Parallelism, tt.wantParallel)
			}
		})
	}
}

func TestScorePipeline_EmptyYAML(t *testing.T) {
	_, err := cicd.ScorePipeline(context.Background(), "empty", "")
	if err == nil {
		t.Fatal("expected error for empty YAML")
	}
}

func TestScorePipeline_GitLabCI(t *testing.T) {
	yaml := `
stages:
  - test
  - deploy
test:
  stage: test
  script:
    - go test ./...
  cache:
    paths:
      - .go/pkg
deploy_prod:
  stage: deploy
  when: manual
  script:
    - deploy --tag v1.0.0
`
	result, err := cicd.ScorePipeline(context.Background(), "gitlab-svc", yaml)
	if err != nil {
		t.Fatalf("ScorePipeline: %v", err)
	}
	if result.Dimensions.Caching != 1 {
		t.Errorf("Caching = %d, want 1", result.Dimensions.Caching)
	}
	if result.Dimensions.TestGate != 2 {
		t.Errorf("TestGate = %d, want 2", result.Dimensions.TestGate)
	}
	if result.Dimensions.DeployControls != 2 {
		t.Errorf("DeployControls = %d, want 2", result.Dimensions.DeployControls)
	}
}
