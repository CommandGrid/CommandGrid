#!/usr/bin/env bash
# setup.sh — Bootstrap the agent sandbox system.
#
# Expects all three repos as sibling directories:
#   ../llm-proxy/
#   ../sandbox-image/
#   ../control-plane/   (this repo)
#
# What it does:
#   1. Checks prerequisites (Go, Docker)
#   2. Builds llm-proxy
#   3. Builds the sandbox-image Docker image
#   4. Builds control-plane
#   5. Walks you through adding your Anthropic API key
#   6. Copies the hello-world example into ./my-first-sandbox/

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

log()  { echo -e "${GREEN}>>>${NC} $*"; }
warn() { echo -e "${YELLOW}>>>${NC} $*"; }
fail() { echo -e "${RED}>>> FATAL:${NC} $*"; exit 1; }
step() { echo -e "\n${CYAN}${BOLD}--- $* ---${NC}\n"; }

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PARENT_DIR="$(dirname "$SCRIPT_DIR")"

LLM_PROXY_DIR="$PARENT_DIR/llm-proxy"
SANDBOX_IMAGE_DIR="$PARENT_DIR/sandbox-image"
CONTROL_PLANE_DIR="$SCRIPT_DIR"

# ─── Prerequisites ────────────────────────────────────────────────────────────

step "Checking prerequisites"

if ! command -v go &>/dev/null; then
    fail "Go is not installed. Install Go 1.25+ or use 'nix develop' in each repo."
fi

GO_VERSION=$(go version | grep -oE 'go[0-9]+\.[0-9]+' | head -1)
log "Go: $GO_VERSION"

if ! command -v docker &>/dev/null; then
    fail "Docker is not installed. Install Docker Desktop or Docker Engine."
fi

if ! docker info &>/dev/null 2>&1; then
    fail "Docker daemon is not running. Start Docker and try again."
fi

log "Docker: $(docker --version | head -1)"

# Check sibling repos exist.
[[ -d "$LLM_PROXY_DIR" ]] || fail "llm-proxy repo not found at $LLM_PROXY_DIR"
[[ -d "$SANDBOX_IMAGE_DIR" ]] || fail "sandbox-image repo not found at $SANDBOX_IMAGE_DIR"

log "All prerequisites met"

# ─── Build llm-proxy ──────────────────────────────────────────────────────────

step "Building llm-proxy"

cd "$LLM_PROXY_DIR"
make build
log "Built: $LLM_PROXY_DIR/build/llm-proxy"

# ─── Build sandbox-image ─────────────────────────────────────────────────────

step "Building sandbox-image Docker image"

cd "$SANDBOX_IMAGE_DIR"
make image-local
log "Built: sandbox-image:latest"

# ─── Build control-plane ─────────────────────────────────────────────────────

step "Building control-plane"

cd "$CONTROL_PLANE_DIR"
make build
log "Built: $CONTROL_PLANE_DIR/build/control-plane"

CP="$CONTROL_PLANE_DIR/build/control-plane"

# ─── Store Anthropic API key ─────────────────────────────────────────────────

step "Setting up credentials"

# Check if the key is already stored.
if "$CP" secrets list 2>/dev/null | grep -q "anthropic_key"; then
    log "anthropic_key already in secret store, skipping"
else
    echo -e "${BOLD}Enter your Anthropic API key (starts with sk-ant-):${NC}"
    read -rsp "> " ANTHROPIC_KEY
    echo ""

    if [[ -z "$ANTHROPIC_KEY" ]]; then
        warn "No key entered. You can add it later with:"
        warn "  $CP secrets add --name anthropic_key --value 'sk-ant-...'"
    else
        "$CP" secrets add --name anthropic_key --value "$ANTHROPIC_KEY"
        log "Stored anthropic_key in secret store"
    fi
fi

# ─── Copy hello-world example ────────────────────────────────────────────────

step "Setting up hello-world example"

EXAMPLE_DIR="$CONTROL_PLANE_DIR/my-first-sandbox"

if [[ -d "$EXAMPLE_DIR" ]]; then
    warn "$EXAMPLE_DIR already exists, skipping copy"
else
    cp -r "$CONTROL_PLANE_DIR/examples/hello-world" "$EXAMPLE_DIR"
    log "Copied hello-world example to $EXAMPLE_DIR"
fi

# ─── Done ─────────────────────────────────────────────────────────────────────

step "Setup complete"

echo -e "
${BOLD}What just happened:${NC}
  - Built llm-proxy, sandbox-image, and control-plane
  - Stored your Anthropic API key in ~/.config/control-plane/secrets/
  - Created my-first-sandbox/ with a ready-to-run example

${BOLD}To run the hello-world example:${NC}

  ${CYAN}# Terminal 1: start the LLM proxy${NC}
  $LLM_PROXY_DIR/build/llm-proxy -addr :8090

  ${CYAN}# Terminal 2: boot the sandbox${NC}
  cd $EXAMPLE_DIR
  $CP up --name hello-world

  ${CYAN}# When done:${NC}
  $CP status
  $CP down --id <container-id>

${BOLD}Or run it all at once:${NC}
  cd $EXAMPLE_DIR && ./run.sh
"
