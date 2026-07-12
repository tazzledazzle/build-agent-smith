// Package api exposes the /audit/trigger REST endpoint.
package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/tazzledazzle/build-agent-smith/internal/domain"
)

// Runner executes an audit cycle.
type Runner interface {
	Run(ctx context.Context, scope domain.Scope, repos []domain.RepoConfig, target string) (*domain.AuditState, error)
}

// TriggerRequest is the JSON body for POST /audit/trigger.
type TriggerRequest struct {
	Scope string `json:"scope"`
	Repo  string `json:"repo"`
}

// TriggerResponse is returned after a successful audit trigger.
type TriggerResponse struct {
	AuditRunID   string `json:"audit_run_id"`
	Status       string `json:"status"`
	FindingCount int    `json:"finding_count"`
	Scope        string `json:"scope"`
}

// Handler serves audit HTTP endpoints.
type Handler struct {
	runner   Runner
	manifest []domain.RepoConfig
	mux      *http.ServeMux
}

// NewHandler creates an HTTP handler with /audit/trigger registered.
func NewHandler(runner Runner, manifest []domain.RepoConfig) *Handler {
	h := &Handler{runner: runner, manifest: manifest, mux: http.NewServeMux()}
	h.mux.HandleFunc("/audit/trigger", h.trigger)
	return h
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func (h *Handler) trigger(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req TriggerRequest
	if r.Body != nil && r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}
	}
	if req.Scope == "" {
		req.Scope = string(domain.ScopeFull)
	}

	scope := domain.Scope(req.Scope)
	switch scope {
	case domain.ScopeFull, domain.ScopeFinOpsOnly, domain.ScopeIncremental:
	default:
		http.Error(w, "invalid scope", http.StatusBadRequest)
		return
	}
	if scope == domain.ScopeIncremental && req.Repo == "" {
		http.Error(w, "incremental scope requires repo", http.StatusBadRequest)
		return
	}

	state, err := h.runner.Run(r.Context(), scope, h.manifest, req.Repo)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := TriggerResponse{
		AuditRunID:   state.AuditRunID,
		Status:       state.Status,
		FindingCount: len(state.Findings),
		Scope:        string(state.Scope),
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
