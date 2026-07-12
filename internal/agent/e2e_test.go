package agent_test

import (
	"context"
	"testing"

	"github.com/tazzledazzle/build-agent-smith/internal/agent"
	"github.com/tazzledazzle/build-agent-smith/internal/config"
	"github.com/tazzledazzle/build-agent-smith/internal/demo"
	"github.com/tazzledazzle/build-agent-smith/internal/domain"
	"github.com/tazzledazzle/build-agent-smith/internal/output"
	"github.com/tazzledazzle/build-agent-smith/internal/store"
)

func TestEndToEnd_FullAuditWithDemoSources(t *testing.T) {
	manifest, err := config.LoadManifest("../../configs/repos.json")
	if err != nil {
		t.Fatalf("manifest: %v", err)
	}

	mem := &store.Memory{}
	writer := output.New(mem, nil)
	a := agent.New(demo.Sources{})

	state, err := a.Run(context.Background(), agent.RunRequest{
		Scope: domain.ScopeFull,
		Repos: manifest.Repos,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if err := writer.Write(context.Background(), state); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if len(state.Findings) == 0 {
		t.Fatal("expected findings from demo estate")
	}
	if len(mem.Runs) != 1 {
		t.Fatalf("persisted runs = %d", len(mem.Runs))
	}
	if mem.Runs[0].TotalEstimatedWasteUSD <= 0 {
		t.Error("expected positive cloud waste from demo inventory")
	}
	if len(state.RepoScores) != len(manifest.Repos) {
		t.Errorf("repo scores = %d, want %d", len(state.RepoScores), len(manifest.Repos))
	}
}
