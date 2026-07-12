#!/usr/bin/env bash
# Smoke-test: build agent, boot HTTP server, exercise POST /audit/trigger scopes.
# Mirrors the make run + curl behavior documented for local demos.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

ADDR="${AUDIT_ADDR:-127.0.0.1:18081}"
MANIFEST="${AUDIT_MANIFEST:-configs/repos.json}"
BIN="${AUDIT_BIN:-bin/agent}"
TIMEOUT_SECS="${AUDIT_READY_TIMEOUT:-15}"

cleanup() {
  if [[ -n "${AGENT_PID:-}" ]] && kill -0 "$AGENT_PID" 2>/dev/null; then
    kill "$AGENT_PID" 2>/dev/null || true
    wait "$AGENT_PID" 2>/dev/null || true
  fi
}
trap cleanup EXIT

echo "==> building agent"
go build -o "$BIN" ./cmd/agent

echo "==> starting agent on ${ADDR}"
"$BIN" -addr "$ADDR" -manifest "$MANIFEST" >"${TMPDIR:-/tmp}/agent-smith-smoke.log" 2>&1 &
AGENT_PID=$!

ready=0
for ((i = 0; i < TIMEOUT_SECS * 10; i++)); do
  if curl -sf "http://${ADDR}/audit/trigger" -o /dev/null -w '' -X POST \
      -H 'Content-Type: application/json' \
      -d '{"scope":"finops_only"}' 2>/dev/null; then
    ready=1
    break
  fi
  if ! kill -0 "$AGENT_PID" 2>/dev/null; then
    echo "ERROR: agent exited before becoming ready" >&2
    cat "${TMPDIR:-/tmp}/agent-smith-smoke.log" >&2 || true
    exit 1
  fi
  sleep 0.1
done

if [[ "$ready" -ne 1 ]]; then
  echo "ERROR: agent did not become ready within ${TIMEOUT_SECS}s" >&2
  cat "${TMPDIR:-/tmp}/agent-smith-smoke.log" >&2 || true
  exit 1
fi

assert_json_field() {
  local body="$1" field="$2" expect="$3"
  local got
  got="$(python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('$field',''))" <<<"$body")"
  if [[ "$got" != "$expect" ]]; then
    echo "ERROR: expected $field=$expect, got $got" >&2
    echo "$body" >&2
    exit 1
  fi
}

assert_status() {
  local body="$1" min_findings="${2:-0}"
  local status count
  status="$(python3 -c "import json,sys; print(json.load(sys.stdin)['status'])" <<<"$body")"
  count="$(python3 -c "import json,sys; print(json.load(sys.stdin)['finding_count'])" <<<"$body")"
  if [[ "$status" != "COMPLETE" && "$status" != "PARTIAL_AUDIT" ]]; then
    echo "ERROR: unexpected status=$status" >&2
    echo "$body" >&2
    exit 1
  fi
  if [[ "$count" -lt "$min_findings" ]]; then
    echo "ERROR: finding_count=$count, want >= $min_findings" >&2
    echo "$body" >&2
    exit 1
  fi
  local run_id
  run_id="$(python3 -c "import json,sys; print(json.load(sys.stdin)['audit_run_id'])" <<<"$body")"
  if [[ -z "$run_id" ]]; then
    echo "ERROR: empty audit_run_id" >&2
    exit 1
  fi
}

echo "==> POST full audit"
FULL="$(curl -sf -X POST "http://${ADDR}/audit/trigger" \
  -H 'Content-Type: application/json' \
  -d '{"scope":"full"}')"
assert_json_field "$FULL" scope full
assert_status "$FULL" 1
echo "$FULL" | python3 -m json.tool

echo "==> POST finops_only"
FINOPS="$(curl -sf -X POST "http://${ADDR}/audit/trigger" \
  -H 'Content-Type: application/json' \
  -d '{"scope":"finops_only"}')"
assert_json_field "$FINOPS" scope finops_only
assert_status "$FINOPS" 1
echo "$FINOPS" | python3 -m json.tool

echo "==> POST incremental"
INCR="$(curl -sf -X POST "http://${ADDR}/audit/trigger" \
  -H 'Content-Type: application/json' \
  -d '{"scope":"incremental","repo":"payments-api"}')"
assert_json_field "$INCR" scope incremental
assert_status "$INCR" 0
echo "$INCR" | python3 -m json.tool

echo "==> POST incremental without repo (expect 400)"
HTTP_CODE="$(curl -s -o /tmp/agent-smith-incr-err.json -w '%{http_code}' \
  -X POST "http://${ADDR}/audit/trigger" \
  -H 'Content-Type: application/json' \
  -d '{"scope":"incremental"}')"
if [[ "$HTTP_CODE" != "400" ]]; then
  echo "ERROR: expected HTTP 400 for incremental without repo, got $HTTP_CODE" >&2
  cat /tmp/agent-smith-incr-err.json >&2 || true
  exit 1
fi

echo "==> GET /audit/trigger (expect 405)"
HTTP_CODE="$(curl -s -o /dev/null -w '%{http_code}' "http://${ADDR}/audit/trigger")"
if [[ "$HTTP_CODE" != "405" ]]; then
  echo "ERROR: expected HTTP 405 for GET, got $HTTP_CODE" >&2
  exit 1
fi

echo "OK: audit trigger smoke tests passed"
