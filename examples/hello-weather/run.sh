#!/usr/bin/env bash
# run.sh — one-shot runner for hello-weather in proxy mode.
#
# Starts GhostProxy, boots the sandbox via control-plane with Bitwarden secrets,
# tails workflow logs, then tears everything down.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
PARENT_DIR="$(dirname "$REPO_ROOT")"
FLOW_DIR="$PARENT_DIR/FlowSpec/workflows/hello-weather"

CP="$REPO_ROOT/build/control-plane"
PROXY="$PARENT_DIR/GhostProxy/build/ghostproxy"
if [[ ! -f "$PROXY" && -f "$PARENT_DIR/GhostProxy/build/llm-proxy" ]]; then
  PROXY="$PARENT_DIR/GhostProxy/build/llm-proxy"
fi
WORKFLOW_BIN="$SCRIPT_DIR/workflow-hello-weather"

RED='\033[0;31m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

log() { echo -e "${GREEN}>>>${NC} $*"; }
fail() { echo -e "${RED}>>>${NC} $*"; exit 1; }

cleanup() {
  log "Cleaning up..."
  if [[ -n "${SANDBOX_ID:-}" ]]; then
    "$CP" down --secrets-provider bitwarden --config "$SCRIPT_DIR/sandbox.yaml" --id "$SANDBOX_ID" >/dev/null 2>&1 || true
  fi
  if [[ -n "${PROXY_PID:-}" ]]; then
    kill "$PROXY_PID" >/dev/null 2>&1 || true
    wait "$PROXY_PID" 2>/dev/null || true
  fi
  log "Done"
}
trap cleanup EXIT

[[ -f "$CP" ]] || fail "control-plane not built. Run: make build"
[[ -f "$PROXY" ]] || fail "ghostproxy not built. Run: (cd ../GhostProxy && make build)"
[[ -n "${BW_SESSION:-}" ]] || fail "BW_SESSION is not set. Run: export BW_SESSION=\"\$(bw unlock --raw)\""

if [[ ! -x "$WORKFLOW_BIN" ]]; then
  log "Building hello-weather workflow binary for sandbox..."
  [[ -d "$FLOW_DIR" ]] || fail "FlowSpec workflow directory not found: $FLOW_DIR"
  GOOS=linux GOARCH=amd64 go -C "$FLOW_DIR" build -o "$WORKFLOW_BIN" .
  chmod +x "$WORKFLOW_BIN"
fi

echo -e "${BOLD}${CYAN}=== Hello Weather: Workflow Sandbox Demo ===${NC}"
echo ""

log "Starting GhostProxy on :8090..."
"$PROXY" -addr :8090 &
PROXY_PID=$!
sleep 1

if ! curl -sf http://localhost:8090/v1/health >/dev/null; then
  fail "ghostproxy failed to start"
fi
log "GhostProxy is running (pid=$PROXY_PID)"

log "Booting hello-weather sandbox..."
OUTPUT=$("$CP" up --secrets-provider bitwarden --config "$SCRIPT_DIR/sandbox.yaml" --name hello-weather 2>&1) || {
  echo "$OUTPUT"
  fail "Failed to boot sandbox"
}
echo "$OUTPUT"

SANDBOX_ID=$(echo "$OUTPUT" | perl -ne 'if(/id=([a-f0-9]+)/){print $1; exit}')
[[ -n "${SANDBOX_ID:-}" ]] || fail "Sandbox ID not found in output"

log "Sandbox started: $SANDBOX_ID"
log "Tailing container logs (Ctrl+C to stop)..."
echo ""
docker logs -f "$SANDBOX_ID" 2>&1 || true
