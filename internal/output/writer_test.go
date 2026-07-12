package output_test

import (
	"context"
	"testing"

	"github.com/tazzledazzle/build-agent-smith/internal/domain"
	"github.com/tazzledazzle/build-agent-smith/internal/output"
)

type memStore struct {
	runs     []domain.AuditRun
	scores   []domain.RepoScore
	findings []domain.Finding
}

func (m *memStore) SaveAuditRun(_ context.Context, run domain.AuditRun) error {
	m.runs = append(m.runs, run)
	return nil
}

func (m *memStore) SaveRepoScores(_ context.Context, scores []domain.RepoScore) error {
	m.scores = append(m.scores, scores...)
	return nil
}

func (m *memStore) SaveFindings(_ context.Context, findings []domain.Finding) error {
	m.findings = append(m.findings, findings...)
	return nil
}

type memSlack struct {
	messages []string
}

func (s *memSlack) PostDigest(_ context.Context, text string) error {
	s.messages = append(s.messages, text)
	return nil
}

func TestWriter_PersistsAndNotifies(t *testing.T) {
	store := &memStore{}
	slack := &memSlack{}
	w := output.New(store, slack)

	state := &domain.AuditState{
		AuditRunID: "run-1",
		Scope:      domain.ScopeFull,
		Repos:      []domain.RepoConfig{{Name: "a"}, {Name: "b"}},
		Findings: []domain.Finding{
			{Title: "idle", EstimatedCostUSD: 100, Severity: domain.SeverityHigh, PriorityScore: 50},
			{Title: "cov", EstimatedCostUSD: 0, Severity: domain.SeverityMedium, PriorityScore: 5},
		},
		RepoScores: map[string]domain.RepoScore{
			"a": {RepoName: "a", CICDMaturityScore: 8},
		},
		Status: "COMPLETE",
	}

	err := w.Write(context.Background(), state)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if len(store.runs) != 1 {
		t.Fatalf("runs = %d, want 1", len(store.runs))
	}
	if store.runs[0].TotalFindings != 2 {
		t.Errorf("TotalFindings = %d, want 2", store.runs[0].TotalFindings)
	}
	if store.runs[0].TotalEstimatedWasteUSD != 100 {
		t.Errorf("TotalEstimatedWasteUSD = %v, want 100", store.runs[0].TotalEstimatedWasteUSD)
	}
	if store.runs[0].CompletedAt.IsZero() {
		t.Error("CompletedAt should be set")
	}
	if len(store.findings) != 2 {
		t.Errorf("findings saved = %d, want 2", len(store.findings))
	}
	if len(store.scores) != 1 {
		t.Errorf("scores saved = %d, want 1", len(store.scores))
	}
	if len(slack.messages) != 1 {
		t.Fatal("expected slack digest")
	}
}

func TestWriter_EmptyState(t *testing.T) {
	store := &memStore{}
	w := output.New(store, &memSlack{})
	err := w.Write(context.Background(), &domain.AuditState{
		AuditRunID: "run-2",
		Scope:      domain.ScopeFinOpsOnly,
		Status:     "COMPLETE",
	})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if store.runs[0].RepoCount != 0 {
		t.Errorf("RepoCount = %d", store.runs[0].RepoCount)
	}
}
