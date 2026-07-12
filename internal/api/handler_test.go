package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tazzledazzle/build-agent-smith/internal/api"
	"github.com/tazzledazzle/build-agent-smith/internal/domain"
)

type fakeRunner struct {
	lastScope domain.Scope
	lastRepos []domain.RepoConfig
	state     *domain.AuditState
	err       error
}

func (f *fakeRunner) Run(_ context.Context, scope domain.Scope, repos []domain.RepoConfig, target string) (*domain.AuditState, error) {
	f.lastScope = scope
	f.lastRepos = repos
	_ = target
	return f.state, f.err
}

func TestTrigger_FullAudit(t *testing.T) {
	runner := &fakeRunner{state: &domain.AuditState{
		AuditRunID: "abc",
		Status:     "COMPLETE",
		Findings:   []domain.Finding{{Title: "x"}},
	}}
	h := api.NewHandler(runner, []domain.RepoConfig{{Name: "a", Owner: "acme"}})

	body, _ := json.Marshal(map[string]string{"scope": "full"})
	req := httptest.NewRequest(http.MethodPost, "/audit/trigger", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
	if runner.lastScope != domain.ScopeFull {
		t.Errorf("scope = %q", runner.lastScope)
	}
	var resp api.TriggerResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.AuditRunID != "abc" || resp.FindingCount != 1 {
		t.Errorf("resp = %+v", resp)
	}
}

func TestTrigger_FinOpsOnly(t *testing.T) {
	runner := &fakeRunner{state: &domain.AuditState{AuditRunID: "f1", Status: "COMPLETE"}}
	h := api.NewHandler(runner, []domain.RepoConfig{{Name: "a"}})

	body, _ := json.Marshal(map[string]string{"scope": "finops_only"})
	req := httptest.NewRequest(http.MethodPost, "/audit/trigger", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	if runner.lastScope != domain.ScopeFinOpsOnly {
		t.Errorf("scope = %q", runner.lastScope)
	}
}

func TestTrigger_IncrementalRequiresRepo(t *testing.T) {
	runner := &fakeRunner{state: &domain.AuditState{AuditRunID: "i1", Status: "COMPLETE"}}
	h := api.NewHandler(runner, []domain.RepoConfig{{Name: "a"}})

	body, _ := json.Marshal(map[string]string{"scope": "incremental"})
	req := httptest.NewRequest(http.MethodPost, "/audit/trigger", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestTrigger_MethodNotAllowed(t *testing.T) {
	h := api.NewHandler(&fakeRunner{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/audit/trigger", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d", rr.Code)
	}
}
