// Package output persists audit results and emits Slack digests.
package output

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/tazzledazzle/build-agent-smith/internal/domain"
)

// Store persists audit runs, repo scores, and findings.
type Store interface {
	SaveAuditRun(ctx context.Context, run domain.AuditRun) error
	SaveRepoScores(ctx context.Context, scores []domain.RepoScore) error
	SaveFindings(ctx context.Context, findings []domain.Finding) error
}

// Notifier sends digest messages (e.g. Slack).
type Notifier interface {
	PostDigest(ctx context.Context, text string) error
}

// Writer is the output_writer node.
type Writer struct {
	store    Store
	notifier Notifier
}

// New creates a Writer.
func New(store Store, notifier Notifier) *Writer {
	return &Writer{store: store, notifier: notifier}
}

// Write persists state and posts an executive digest.
func (w *Writer) Write(ctx context.Context, state *domain.AuditState) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("output write: %w", err)
	}

	var waste float64
	for _, f := range state.Findings {
		waste += f.EstimatedCostUSD
	}

	now := time.Now().UTC()
	run := domain.AuditRun{
		ID:                     state.AuditRunID,
		TriggeredAt:            now,
		Scope:                  state.Scope,
		RepoCount:              len(state.Repos),
		TotalFindings:          len(state.Findings),
		TotalEstimatedWasteUSD: waste,
		CompletedAt:            now,
		Status:                 state.Status,
	}
	if err := w.store.SaveAuditRun(ctx, run); err != nil {
		return fmt.Errorf("save audit run: %w", err)
	}

	scores := make([]domain.RepoScore, 0, len(state.RepoScores))
	for _, s := range state.RepoScores {
		s.AuditRunID = state.AuditRunID
		scores = append(scores, s)
	}
	if err := w.store.SaveRepoScores(ctx, scores); err != nil {
		return fmt.Errorf("save repo scores: %w", err)
	}
	if err := w.store.SaveFindings(ctx, state.Findings); err != nil {
		return fmt.Errorf("save findings: %w", err)
	}

	if w.notifier != nil {
		digest := buildDigest(state, waste)
		if err := w.notifier.PostDigest(ctx, digest); err != nil {
			return fmt.Errorf("slack digest: %w", err)
		}
	}
	return nil
}

func buildDigest(state *domain.AuditState, waste float64) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Platform Audit %s (%s)\n", state.AuditRunID, state.Scope)
	fmt.Fprintf(&b, "Findings: %d | Estimated waste: $%.2f | Status: %s\n",
		len(state.Findings), waste, state.Status)
	limit := 10
	if len(state.Findings) < limit {
		limit = len(state.Findings)
	}
	for i := 0; i < limit; i++ {
		f := state.Findings[i]
		fmt.Fprintf(&b, "- [%s] %s (priority %.1f)\n", f.Severity, f.Title, f.PriorityScore)
	}
	return b.String()
}
