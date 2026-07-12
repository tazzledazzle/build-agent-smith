package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tazzledazzle/build-agent-smith/internal/agent"
	"github.com/tazzledazzle/build-agent-smith/internal/api"
	"github.com/tazzledazzle/build-agent-smith/internal/config"
	"github.com/tazzledazzle/build-agent-smith/internal/demo"
	"github.com/tazzledazzle/build-agent-smith/internal/domain"
	"github.com/tazzledazzle/build-agent-smith/internal/output"
	"github.com/tazzledazzle/build-agent-smith/internal/store"
)

// liveRunner wires the real agent + demo sources the way cmd/agent does.
type liveRunner struct {
	agent  *agent.Agent
	writer *output.Writer
}

func (r *liveRunner) Run(ctx context.Context, scope domain.Scope, repos []domain.RepoConfig, target string) (*domain.AuditState, error) {
	state, err := r.agent.Run(ctx, agent.RunRequest{
		Scope:          scope,
		Repos:          repos,
		TargetRepoName: target,
	})
	if err != nil {
		return nil, err
	}
	if err := r.writer.Write(ctx, state); err != nil {
		return nil, err
	}
	return state, nil
}

func newLiveHandler(t *testing.T) (*api.Handler, *store.Memory) {
	t.Helper()
	manifest, err := config.LoadManifest("../../configs/repos.json")
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if len(manifest.Repos) != 5 {
		t.Fatalf("manifest repos = %d, want 5 (make run advertises 5 repos)", len(manifest.Repos))
	}
	mem := &store.Memory{}
	runner := &liveRunner{
		agent:  agent.New(demo.Sources{}),
		writer: output.New(mem, nil),
	}
	return api.NewHandler(runner, manifest.Repos), mem
}

func TestTrigger_LiveScopesMatchSmokeScript(t *testing.T) {
	h, mem := newLiveHandler(t)

	tests := []struct {
		name           string
		body           string
		wantStatus     int
		wantScope      string
		minFindings    int
		expectPersist  bool
	}{
		{
			name:          "full",
			body:          `{"scope":"full"}`,
			wantStatus:    http.StatusOK,
			wantScope:     "full",
			minFindings:   1,
			expectPersist: true,
		},
		{
			name:          "finops_only",
			body:          `{"scope":"finops_only"}`,
			wantStatus:    http.StatusOK,
			wantScope:     "finops_only",
			minFindings:   1,
			expectPersist: true,
		},
		{
			name:          "incremental payments-api",
			body:          `{"scope":"incremental","repo":"payments-api"}`,
			wantStatus:    http.StatusOK,
			wantScope:     "incremental",
			minFindings:   0,
			expectPersist: true,
		},
		{
			name:       "incremental missing repo",
			body:       `{"scope":"incremental"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:          "empty body defaults to full",
			body:          "",
			wantStatus:    http.StatusOK,
			wantScope:     "full",
			minFindings:   1,
			expectPersist: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := len(mem.Runs)
			var req *http.Request
			if tt.body == "" {
				req = httptest.NewRequest(http.MethodPost, "/audit/trigger", http.NoBody)
			} else {
				req = httptest.NewRequest(http.MethodPost, "/audit/trigger", bytes.NewReader([]byte(tt.body)))
				req.Header.Set("Content-Type", "application/json")
			}
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", rr.Code, tt.wantStatus, rr.Body.String())
			}
			if tt.wantStatus != http.StatusOK {
				return
			}

			var resp api.TriggerResponse
			if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if resp.Scope != tt.wantScope {
				t.Errorf("scope = %q, want %q", resp.Scope, tt.wantScope)
			}
			if resp.AuditRunID == "" {
				t.Error("empty audit_run_id")
			}
			if resp.FindingCount < tt.minFindings {
				t.Errorf("finding_count = %d, want >= %d", resp.FindingCount, tt.minFindings)
			}
			if resp.Status != "COMPLETE" && resp.Status != "PARTIAL_AUDIT" {
				t.Errorf("status = %q", resp.Status)
			}
			if tt.expectPersist && len(mem.Runs) <= before {
				t.Error("expected audit run persisted via output writer")
			}
		})
	}
}

func TestTrigger_FinOpsOnlyWastePositive(t *testing.T) {
	h, mem := newLiveHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/audit/trigger",
		bytes.NewReader([]byte(`{"scope":"finops_only"}`)))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
	if len(mem.Runs) == 0 {
		t.Fatal("no runs persisted")
	}
	last := mem.Runs[len(mem.Runs)-1]
	if last.TotalEstimatedWasteUSD <= 0 {
		t.Errorf("TotalEstimatedWasteUSD = %v, want > 0 from demo inventory", last.TotalEstimatedWasteUSD)
	}
	if last.Scope != domain.ScopeFinOpsOnly {
		t.Errorf("Scope = %q", last.Scope)
	}
}
