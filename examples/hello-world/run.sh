#!/usr/bin/env bash
# run.sh — One-shot runner for the hello-world example.
#
# Starts the llm-proxy in the background, boots the sandbox,
# waits for it to finish, then cleans up.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
PARENT="$(dirname "$REPO_ROOT")"

CP="$REPO_ROOT/build/control-plane"
PROXY="$PARENT/llm-proxy/build/llm-proxy"

RED='\033[0;31m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

log() { echo -e "${GREEN}>>>${NC} $*"; }

cleanup() {
    log "Cleaning up..."
    # Stop the sandbox.
    if [[ -n "${SANDBOX_ID:-}" ]]; then
        "$CP" down --config "$SCRIPT_DIR/sandbox.toml" --id "$SANDBOX_ID" 2>/dev/null || true
    fi
    # Stop the proxy.
    if [[ -n "${PROXY_PID:-}" ]]; then
        kill "$PROXY_PID" 2>/dev/null || true
        wait "$PROXY_PID" 2>/dev/null || true
    fi
    log "Done"
}
trap cleanup EXIT

# Check binaries exist.
[[ -f "$CP" ]]    || { echo -e "${RED}control-plane not built. Run: make build${NC}"; exit 1; }
[[ -f "$PROXY" ]] || { echo -e "${RED}llm-proxy not built. Run: cd ../llm-proxy && make build${NC}"; exit 1; }

echo -e "${BOLD}${CYAN}=== Hello World: Agent Sandbox Demo ===${NC}"
echo ""

# Start the proxy in the background.
log "Starting llm-proxy on :8090..."
"$PROXY" -addr :8090 &
PROXY_PID=$!
sleep 1

# Verify proxy is up.
if ! curl -sf http://localhost:8090/v1/health >/dev/null; then
    echo -e "${RED}llm-proxy failed to start${NC}"
    exit 1
fi
log "llm-proxy is running (pid=$PROXY_PID)"

# Boot the sandbox.
log "Booting sandbox..."
cd "$SCRIPT_DIR"
OUTPUT=$("$CP" up --config sandbox.toml --name hello-world 2>&1) || {
    echo "$OUTPUT"
    echo -e "${RED}Failed to boot sandbox${NC}"
    exit 1
}
echo "$OUTPUT"

# Extract the sandbox ID from the output.
SANDBOX_ID=$(echo "$OUTPUT" | grep -oE 'id=[a-f0-9]+' | head -1 | cut -d= -f2)

if [[ -n "$SANDBOX_ID" ]]; then
    log "Sandbox started: $SANDBOX_ID"
    log "Tailing container logs (Ctrl+C to stop)..."
    echo ""
    docker logs -f "$SANDBOX_ID" 2>&1 || true
else
    log "Sandbox may have started — check with: $CP status"
fi
