package store_test

import (
	"context"
	"testing"

	"github.com/tazzledazzle/build-agent-smith/internal/domain"
	"github.com/tazzledazzle/build-agent-smith/internal/store"
)

func TestMemory_PersistsAllEntities(t *testing.T) {
	m := &store.Memory{}
	ctx := context.Background()

	if err := m.SaveAuditRun(ctx, domain.AuditRun{ID: "1", TotalFindings: 2}); err != nil {
		t.Fatal(err)
	}
	if err := m.SaveRepoScores(ctx, []domain.RepoScore{{RepoName: "a"}}); err != nil {
		t.Fatal(err)
	}
	if err := m.SaveFindings(ctx, []domain.Finding{{Title: "x"}}); err != nil {
		t.Fatal(err)
	}

	if len(m.Runs) != 1 || len(m.Scores) != 1 || len(m.Findings) != 1 {
		t.Fatalf("got runs=%d scores=%d findings=%d", len(m.Runs), len(m.Scores), len(m.Findings))
	}
}
