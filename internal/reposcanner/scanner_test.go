package reposcanner_test

import (
	"context"
	"testing"

	"github.com/tazzledazzle/build-agent-smith/internal/domain"
	"github.com/tazzledazzle/build-agent-smith/internal/reposcanner"
)

type fakeRepoClient struct {
	files map[string]bool
}

func (f *fakeRepoClient) HasFile(_ context.Context, _, _, path string) (bool, error) {
	return f.files[path], nil
}

func TestScan_FullyStandardized(t *testing.T) {
	client := &fakeRepoClient{files: map[string]bool{
		".github/workflows/ci.yml": true,
		"Dockerfile":               true,
		".pre-commit-config.yaml":  true,
		"CODEOWNERS":               true,
		".golangci.yml":            true,
	}}
	scanner := reposcanner.New(client)

	result, err := scanner.Scan(context.Background(), domain.RepoConfig{
		Name: "payments-api", Owner: "acme", Provider: "github",
	})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if result.StandardizationScore != 5 {
		t.Errorf("StandardizationScore = %d, want 5", result.StandardizationScore)
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected no findings, got %d", len(result.Findings))
	}
}

func TestScan_MissingArtifacts(t *testing.T) {
	client := &fakeRepoClient{files: map[string]bool{}}
	scanner := reposcanner.New(client)

	result, err := scanner.Scan(context.Background(), domain.RepoConfig{
		Name: "legacy", Owner: "acme", Provider: "github",
	})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if result.StandardizationScore != 0 {
		t.Errorf("StandardizationScore = %d, want 0", result.StandardizationScore)
	}
	if len(result.Findings) == 0 {
		t.Fatal("expected findings for missing standardization artifacts")
	}
	for _, f := range result.Findings {
		if f.FindingType != domain.FindingTypeStandardization {
			t.Errorf("FindingType = %q, want standardization", f.FindingType)
		}
	}
}

func TestScan_PartialScore(t *testing.T) {
	client := &fakeRepoClient{files: map[string]bool{
		".github/workflows/ci.yml": true,
		"Dockerfile":               true,
	}}
	scanner := reposcanner.New(client)

	result, err := scanner.Scan(context.Background(), domain.RepoConfig{
		Name: "partial", Owner: "acme", Provider: "github",
	})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	// CI config (1) + Dockerfile (1) + branch protection proxy via CI presence is not counted separately
	// Rubric: CI=1, Dockerfile=1, pre-commit=1, CODEOWNERS=1, lint config=1 → max 5
	if result.StandardizationScore != 2 {
		t.Errorf("StandardizationScore = %d, want 2", result.StandardizationScore)
	}
}

func TestScan_GitLabCI(t *testing.T) {
	client := &fakeRepoClient{files: map[string]bool{
		".gitlab-ci.yml":          true,
		"Dockerfile":              true,
		".pre-commit-config.yaml": true,
		"CODEOWNERS":              true,
		".golangci.yml":           true,
	}}
	scanner := reposcanner.New(client)

	result, err := scanner.Scan(context.Background(), domain.RepoConfig{
		Name: "gl-svc", Owner: "acme", Provider: "gitlab",
	})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if result.StandardizationScore != 5 {
		t.Errorf("StandardizationScore = %d, want 5", result.StandardizationScore)
	}
}
