# Testing Patterns

**Analysis Date:** 2026-07-12

## Test Framework

**Runner:**
- Go standard toolchain: `go test`
- No third-party test framework (no testify, gomock, or httpexpect)

**Assertion Library:**
- Built-in `testing` package
- Manual assertions via `t.Fatalf`, `t.Errorf`, `t.Fatal`, `t.Error`
- Equality checked with `!=` / `==`; no assertion helpers library

**Run Commands:**
```bash
make test                              # go test -race -count=1 -coverprofile=coverage.out ./...
make smoke                             # ./scripts/test-audit-trigger.sh
make lint                              # golangci-lint run ./...
make vet                               # go vet ./...
go test ./internal/domain/ -v          # single package
go test ./internal/api/ -run TestTrigger_FullAudit -v   # single test
go tool cover -func=coverage.out       # coverage summary after make test
```

## Test File Organization

**Location:**
- Co-located `*_test.go` next to implementation under `internal/<pkg>/`
- Smoke script outside Go: `scripts/test-audit-trigger.sh` (invoked by `make smoke`)

**Naming:**
- Unit / package tests: `<subject>_test.go` (`handler_test.go`, `auditor_test.go`)
- Broader flows: `e2e_test.go` (`internal/agent/e2e_test.go`), `live_trigger_test.go` (`internal/api/live_trigger_test.go`)
- Test functions: `Test<TypeOrFunc>_<Scenario>` — e.g. `TestScan_FullyStandardized`, `TestPlan_FinOpsOnly`, `TestTrigger_IncrementalRequiresRepo`

**Structure:**
```
internal/
  domain/
    types.go
    domain_test.go          # table-driven pure functions
  api/
    handler.go
    handler_test.go         # httptest + fakeRunner
    live_trigger_test.go    # real agent + demo sources
  agent/
    agent.go
    agent_test.go           # stubDeps
    e2e_test.go             # full audit + store write
  reposcanner/
    scanner.go
    scanner_test.go         # fakeRepoClient
scripts/
  test-audit-trigger.sh     # process-level smoke
```

## Test Structure

**Suite Organization:**
```go
package domain_test

import (
	"testing"

	"github.com/tazzledazzle/build-agent-smith/internal/domain"
)

func TestPriorityScore(t *testing.T) {
	tests := []struct {
		name             string
		costImpactUSD    float64
		severity         domain.Severity
		remediationHours float64
		want             float64
	}{
		{name: "critical high cost low effort", costImpactUSD: 1000, severity: domain.SeverityCritical, remediationHours: 2, want: 2000},
		{name: "zero effort uses minimum of one hour", costImpactUSD: 500, severity: domain.SeverityHigh, remediationHours: 0, want: 1500},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := domain.PriorityScore(tt.costImpactUSD, tt.severity, tt.remediationHours)
			if got != tt.want {
				t.Errorf("PriorityScore() = %v, want %v", got, tt.want)
			}
		})
	}
}
```

**Patterns:**
- Prefer **external test packages** (`package foo_test`) to exercise exported API only — used across all `internal/*_test.go` files
- Table-driven tests with `tests := []struct{ name string; ... }` + `t.Run(tt.name, ...)` for multi-case logic (`domain`, `cicd`, `api` live triggers)
- Scenario-named standalone `Test*` functions when setup differs heavily (`TestScan_MissingArtifacts`, `TestAgent_RespectsCancellation`)
- Fail fast on unexpected errors: `if err != nil { t.Fatalf("Op: %v", err) }`
- Use `t.Helper()` on shared test constructors (`newLiveHandler` in `internal/api/live_trigger_test.go`)
- Use `t.TempDir()` for filesystem fixtures (`internal/config/manifest_test.go`)

## Mocking

**Framework:**
- No mocking library — hand-written fakes/stubs implementing package interfaces

**Patterns:**
```go
// Fake implementing an interface (api.Runner)
type fakeRunner struct {
	lastScope domain.Scope
	state     *domain.AuditState
	err       error
}

func (f *fakeRunner) Run(_ context.Context, scope domain.Scope, repos []domain.RepoConfig, target string) (*domain.AuditState, error) {
	f.lastScope = scope
	return f.state, f.err
}

// Stub for agent.Dependencies
type stubDeps struct {
	files map[string]map[string]bool
	yamls map[string]string
	cov   map[string]agent.CoverageInput
	cloud finops.Inventory
}

func (s *stubDeps) HasFile(_ context.Context, _, repo, path string) (bool, error) {
	if m, ok := s.files[repo]; ok {
		return m[path], nil
	}
	return false, nil
}
// ... FetchPipelineYAML, FetchCoverage, FetchCloudInventory
```

**What to Mock (fake/stub):**
- Interfaces at package boundaries: `api.Runner`, `reposcanner.FileChecker`, `agent.Dependencies`, `output.Store`, `output.Notifier`
- Unused `context.Context` parameters: name `_` when cancellation is not under test
- Compose stubs when counting calls (`countingDeps` embeds `stubDeps` + `atomic.Int64`)

**What NOT to Mock:**
- Pure domain/scoring logic (`domain.PriorityScore`, `cicd.ScorePipeline`, `finops.Analyze`, `aggregator.Aggregate`)
- In-memory `store.Memory` when testing persistence wiring
- `demo.Sources` in e2e / live trigger tests (use real demo data)

## Fixtures and Factories

**Test Data:**
```go
// Inline YAML / JSON fixtures in the test (preferred for pipeline scoring)
yaml := `
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - run: go test ./...
`

// Temp files for config loading
dir := t.TempDir()
path := filepath.Join(dir, "repos.json")
_ = os.WriteFile(path, []byte(`{"repos":[{"name":"payments-api","owner":"acme","provider":"github","default_branch":"main"}]}`), 0o644)

// Checked-in manifest for integration/e2e
manifest, err := config.LoadManifest("../../configs/repos.json")
```

**Location:**
- Small fixtures: inline in the test function or table row
- Shared estate data: `configs/repos.json` + `internal/demo/sources.go`
- No separate `testdata/` directories detected — add `testdata/` only when fixtures grow large

## Coverage

**Requirements:**
- No enforced minimum percentage in Makefile or CI config in-repo
- `make test` always writes `coverage.out` (`-coverprofile=coverage.out`)
- `coverage.out` is gitignored

**Configuration:**
- Flags: `-race -count=1 -coverprofile=coverage.out ./...`
- `-race` is required for parallel agent worker tests
- `-count=1` disables test result caching for deterministic local runs

**View Coverage:**
```bash
make test
go tool cover -func=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

## Test Types

**Unit Tests:**
- Single package, fake/stub deps, no network
- Examples: `internal/domain/domain_test.go`, `internal/cicd/auditor_test.go`, `internal/reposcanner/scanner_test.go`, `internal/finops/analyzer_test.go`, `internal/api/handler_test.go`

**Integration Tests:**
- Multiple real packages wired together with demo sources or memory store
- Examples: `internal/agent/agent_test.go`, `internal/agent/e2e_test.go`, `internal/api/live_trigger_test.go`, `internal/config/repos_manifest_test.go`

**E2E / Smoke:**
- Process-level: `scripts/test-audit-trigger.sh` builds `bin/agent`, boots HTTP server, curls `POST /audit/trigger` for scopes `full`, `finops_only`, `incremental`, plus 400/405 cases
- Go e2e: `TestEndToEnd_FullAuditWithDemoSources` runs agent → output writer → memory store
- Live HTTP: `TestTrigger_LiveScopesMatchSmokeScript` mirrors smoke script expectations inside `go test`

## Common Patterns

**Async / Concurrency Testing:**
```go
func TestAgent_RespectsCancellation(t *testing.T) {
	a := agent.New(deps)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := a.Run(ctx, agent.RunRequest{ /* ... */ })
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
}

func TestAgent_ParallelExecutionCompletes(t *testing.T) {
	start := time.Now()
	state, err := a.Run(context.Background(), agent.RunRequest{ /* multi-repo */ })
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if time.Since(start) > 5*time.Second {
		t.Error("full audit of 2 repos took too long")
	}
	_ = state
}
```

**Error Testing:**
```go
func TestScorePipeline_EmptyYAML(t *testing.T) {
	_, err := cicd.ScorePipeline(context.Background(), "empty", "")
	if err == nil {
		t.Fatal("expected error for empty YAML")
	}
}

func TestTrigger_IncrementalRequiresRepo(t *testing.T) {
	// httptest POST without repo field
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}
```

**HTTP Handler Testing:**
```go
body, _ := json.Marshal(map[string]string{"scope": "full"})
req := httptest.NewRequest(http.MethodPost, "/audit/trigger", bytes.NewReader(body))
rr := httptest.NewRecorder()
h.ServeHTTP(rr, req)
if rr.Code != http.StatusOK {
	t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
}
```

**Snapshot Testing:**
- Not used — assert concrete fields (`TotalScore`, `FindingType`, `FindingCount`, status codes)

**When adding tests for new code:**
1. Place `*_test.go` beside the package; use `package <name>_test`
2. Fake interfaces at the boundary; do not invent a mock library
3. Prefer table-driven cases for scoring/validation matrices
4. Keep `make test` green with `-race`; add smoke coverage if you change `/audit/trigger` behavior

---

*Testing analysis: 2026-07-12*
*Update when test patterns change*
