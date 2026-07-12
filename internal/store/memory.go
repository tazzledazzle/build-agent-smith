// Package store provides persistence adapters for audit outputs.
package store

import (
	"context"
	"sync"

	"github.com/tazzledazzle/build-agent-smith/internal/domain"
)

// Memory is an in-process Store for tests and local runs.
type Memory struct {
	mu       sync.Mutex
	Runs     []domain.AuditRun
	Scores   []domain.RepoScore
	Findings []domain.Finding
}

// SaveAuditRun implements output.Store.
func (m *Memory) SaveAuditRun(_ context.Context, run domain.AuditRun) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Runs = append(m.Runs, run)
	return nil
}

// SaveRepoScores implements output.Store.
func (m *Memory) SaveRepoScores(_ context.Context, scores []domain.RepoScore) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Scores = append(m.Scores, scores...)
	return nil
}

// SaveFindings implements output.Store.
func (m *Memory) SaveFindings(_ context.Context, findings []domain.Finding) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Findings = append(m.Findings, findings...)
	return nil
}
