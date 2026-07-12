# Coding Conventions

**Analysis Date:** 2026-07-12

## Naming Patterns

**Files:**
- Lowercase single-word package directories under `internal/` (`agent`, `cicd`, `finops`, `reposcanner`)
- Implementation: descriptive noun (`handler.go`, `scanner.go`, `auditor.go`, `analyzer.go`, `writer.go`, `manifest.go`, `types.go`)
- Tests: `*_test.go` co-located with the package under test (`handler_test.go`, `scanner_test.go`)
- Entry point: `cmd/agent/main.go`

**Functions:**
- Exported constructors: `New`, `NewHandler` (`internal/agent/agent.go`, `internal/api/handler.go`)
- Exported operations: verb phrases — `Run`, `Plan`, `Scan`, `Analyze`, `ScorePipeline`, `Aggregate`, `Write`, `LoadManifest`
- Unexported helpers: camelCase verbs — `toSet`, `dedupe`, `clamp`, `buildDigest`, `gapFinding`, `cloudFinding`
- HTTP method handlers: short lowercase — `trigger` on `*Handler`
- No `Get`/`Set` prefixes; field access is direct on structs

**Variables:**
- camelCase locals (`auditRunID`, `fileCalls`, `wantCache`)
- Short idiomatic names in narrow scope (`err`, `ctx`, `w`, `r`, `tt`, `mu`, `wg`)
- Unexported package-level vars for tables/config (`dimensionPaths`, `secretPatterns`, `lineThreshold`)

**Types:**
- PascalCase structs and interfaces, no `I` prefix (`Runner`, `Dependencies`, `FileChecker`, `Store`, `Notifier`)
- Domain enums as typed string constants: `Scope`, `Severity`, `FindingType`, `FindingStatus` in `internal/domain/types.go`
- Const values: `ScopeFull`, `SeverityCritical`, `FindingTypeCICDGap` (type prefix + descriptive suffix)
- Result DTOs named `Result`, `Output`, `PlanResult`, `TriggerRequest` / `TriggerResponse`

## Code Style

**Formatting:**
- Standard Go formatting via `gofmt` (`make fmt` → `gofmt -w .`)
- Tabs for indentation (gofmt default)
- Double quotes for strings; raw string literals (`` ` ``) for multi-line YAML fixtures in tests
- No project `.editorconfig` or custom gofmt options detected

**Linting:**
- `go vet ./...` via `make vet`
- `golangci-lint run ./...` via `make lint` (no checked-in `.golangci.yml` — use golangci-lint defaults)
- Run lint/vet before considering a change complete

## Import Organization

**Order (gofmt / goimports groups):**
1. Standard library (`context`, `fmt`, `testing`, `net/http`)
2. Blank line
3. Third-party (`github.com/google/uuid`, `gopkg.in/yaml.v3`)
4. Blank line
5. Internal modules (`github.com/tazzledazzle/build-agent-smith/internal/...`)

**Grouping:**
- Blank line between stdlib, third-party, and internal groups
- Prefer `goimports` / editor organize-imports to keep groups sorted
- No import aliases unless required for collision

**Path Aliases:**
- Not applicable — use full module path `github.com/tazzledazzle/build-agent-smith/internal/<pkg>`

## Error Handling

**Patterns:**
- Return `(T, error)` from exported operations; never panic in library code
- Wrap with context via `fmt.Errorf("operation: %w", err)` — see `internal/config/manifest.go`, `internal/reposcanner/scanner.go`, `internal/output/writer.go`
- Check `ctx.Err()` at the start of long-running node functions (`Plan`, `Scan`, `Analyze`, `Write`)
- HTTP boundary: map errors to status codes with `http.Error` in `internal/api/handler.go` (400 validation, 405 method, 500 runner failure)
- Non-fatal worker failures: append `domain.AuditError` and set `Status` to `"PARTIAL_AUDIT"` rather than failing the whole run (`internal/agent/agent.go`)

**Error Types:**
- Prefer plain `error` with descriptive messages; no custom error types detected
- Validation failures return early with clear messages (`plan: incremental scope requires target repo`)
- Ignore intentional encode errors only when documented (`_ = json.NewEncoder(w).Encode(resp)` after headers set)

## Logging

**Framework:**
- Standard library `log` package in `cmd/agent/main.go` only
- Levels: `log.Printf` for info, `log.Fatalf` for fatal startup/server errors

**Patterns:**
- Log at process boundaries (listen address, shutdown, Slack digest stub)
- Library packages (`internal/*`) do not log — return errors to callers
- Demo Slack notifier prints digests via `log.Printf("slack digest:\n%s", text)`

## Comments

**When to Comment:**
- Package doc comment on every package: `// Package X ...` as the first line of the primary `.go` file
- Exported types and functions get a one-line doc comment starting with the name (`// New creates an Agent...`)
- Inline comments explain non-obvious rubric math or scoring edge cases (e.g. zero-effort floor in `PriorityScore`)
- Avoid restating the obvious next to simple assignments

**GoDoc:**
- Required for all exported identifiers
- Prefer complete sentences; link related types by name in prose

**TODO Comments:**
- None present in the codebase; add `// TODO: description` only when tracking unfinished work, and prefer issues for lasting debt

## Function Design

**Size:**
- Keep exported entry points focused; extract unexported helpers for scoring dimensions (`scoreCaching`, `scoreTestGate`, etc. in `internal/cicd/auditor.go`)
- Prefer early returns for validation and context cancellation

**Parameters:**
- First parameter is `context.Context` for any operation that may block, cancel, or call I/O
- Pass domain structs (`domain.RepoConfig`, `domain.AuditState`) rather than long primitive lists
- Construct request structs for multi-field inputs (`agent.RunRequest`, `supervisor.Request`, `aggregator.Input`)

**Return Values:**
- Prefer pointers for mutable/aggregated results (`*Result`, `*domain.AuditState`)
- Return `nil, err` on failure; never return a partial success pointer with a non-nil error unless documented
- Use named result fields on structs instead of multiple return values beyond `(T, error)`

## Module Design

**Exports:**
- Narrow public surface per package: constructors + one primary operation
- Define small consumer-owned interfaces at the call site (`api.Runner`, `output.Store`, `output.Notifier`, `reposcanner.FileChecker`, `agent.Dependencies`)
- Accept interfaces, return concrete types (`*Agent`, `*Handler`, `*Writer`)

**Barrel Files:**
- Not used — import the specific `internal/<pkg>` package
- Shared types live only in `internal/domain`; workers must not import each other sideways when domain types suffice
- `cmd/agent` wires dependencies; keep orchestration out of worker packages

---

*Convention analysis: 2026-07-12*
*Update when patterns change*
