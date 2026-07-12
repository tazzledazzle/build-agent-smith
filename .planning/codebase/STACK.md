# Technology Stack

**Analysis Date:** 2026-07-12

## Languages

**Primary:**
- Go 1.26.2 - All application code under `cmd/` and `internal/`

**Secondary:**
- SQL - Schema definition in `migrations/001_schema.sql` (PostgreSQL dialect; not executed by the Go binary today)
- Bash - Smoke tests in `scripts/test-audit-trigger.sh`
- JSON - Repo audit manifest in `configs/repos.json`
- Markdown - Design/reference docs (`TDD.md`, `README.md`)

## Runtime

**Environment:**
- Go toolchain 1.26.2 (module directive in `go.mod`)
- Standard library `net/http` server — no separate app runtime beyond the compiled binary
- Docker multi-stage build uses `golang:1.26-alpine` (builder) and `alpine:3.20` (runtime) in `Dockerfile`

**Package Manager:**
- Go modules (`go mod`)
- Lockfile: `go.sum` present

## Frameworks

**Core:**
- None — vanilla Go HTTP server (`net/http.ServeMux` in `internal/api/handler.go`)
- Orchestration is a custom Go “LangGraph-equivalent” agent (`internal/agent/agent.go`), not the Python LangGraph library referenced in `TDD.md` / `README.md`

**Testing:**
- Go standard `testing` package — unit, package, and e2e-style tests co-located as `*_test.go`
- Race detector via `go test -race` (`Makefile` target `test`)
- Coverage profiles via `-coverprofile=coverage.out`

**Build/Dev:**
- `go build` — produces `bin/agent` from `cmd/agent` (`Makefile` targets `build`, `run`)
- `gofmt` — formatting (`Makefile` target `fmt`)
- `go vet` — static checks (`Makefile` target `vet`)
- `golangci-lint` — lint entrypoint (`Makefile` target `lint`); no `.golangci.yml` committed
- Docker — container image build (`Dockerfile`)

## Key Dependencies

**Critical:**
- `github.com/google/uuid` v1.6.0 - Audit run IDs in `internal/supervisor/supervisor.go`
- `gopkg.in/yaml.v3` v3.0.1 - CI/CD pipeline YAML parsing in `internal/cicd/auditor.go`

**Infrastructure:**
- Go stdlib `net/http` - HTTP listen/serve and `/audit/trigger` routing (`cmd/agent/main.go`, `internal/api/handler.go`)
- Go stdlib `encoding/json` - Manifest load and API request/response bodies
- Go stdlib `sync` - Parallel worker fan-out in `internal/agent/agent.go`
- No database driver, AWS SDK, Slack SDK, or GitHub/GitLab client modules in `go.mod`

## Configuration

**Environment:**
- No required secrets or `.env` files for the current binary
- CLI flags only (`cmd/agent/main.go`):
  - `-addr` (default `:8080`) — HTTP listen address
  - `-manifest` (default `configs/repos.json`) — path to repo list
- Smoke script optional overrides (`scripts/test-audit-trigger.sh`): `AUDIT_ADDR`, `AUDIT_MANIFEST`, `AUDIT_BIN`, `AUDIT_READY_TIMEOUT`

**Build:**
- `go.mod` / `go.sum` — module identity and dependency pins
- `Dockerfile` — multi-stage static binary (`CGO_ENABLED=0`)
- `Makefile` — test, lint, vet, fmt, build, run, smoke
- `configs/repos.json` — audited repository manifest (name, owner, provider, default_branch)

## Platform Requirements

**Development:**
- Go 1.26.2+ toolchain
- Any OS supported by Go (developed/tested on macOS/Linux)
- Optional: `golangci-lint` for `make lint`, Docker for image builds, `curl` + `python3` for `make smoke`

**Production:**
- Static Linux binary in Alpine container (`Dockerfile`: exposes port 8080, entrypoint `/app/agent`)
- Designed deployment target (per `TDD.md`): Kubernetes CronJob + long-running HTTP for triggers — not wired as manifests in this repo
- Current process wiring uses in-memory store + log-based Slack stub (`cmd/agent/main.go`); no live Postgres/AWS/Slack clients at runtime

---

*Stack analysis: 2026-07-12*
*Update after major dependency changes*
