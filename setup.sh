#!/usr/bin/env bash
# setup.sh — Bootstrap the agent sandbox system.
#
# Expects all three repos as sibling directories:
#   ../GhostProxy/
#   ../RootFS/
#   ../CommandGrid/   (this repo)
#
# What it does:
#   1. Checks prerequisites (Go, Docker)
#   2. Builds GhostProxy
#   3. Builds the rootfs Docker image
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

GHOSTPROXY_DIR="$PARENT_DIR/GhostProxy"
ROOTFS_DIR="$PARENT_DIR/RootFS"
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
[[ -d "$GHOSTPROXY_DIR" ]] || fail "GhostProxy repo not found at $GHOSTPROXY_DIR"
[[ -d "$ROOTFS_DIR" ]] || fail "RootFS repo not found at $ROOTFS_DIR"

log "All prerequisites met"

# ─── Build GhostProxy ─────────────────────────────────────────────────────────

step "Building GhostProxy"

cd "$GHOSTPROXY_DIR"
make build
log "Built: $GHOSTPROXY_DIR/build/ghostproxy"

# ─── Build rootfs ────────────────────────────────────────────────────────────

step "Building rootfs Docker image"

cd "$ROOTFS_DIR"
make image-local
log "Built: rootfs:latest"

# ─── Build control-plane ─────────────────────────────────────────────────────

step "Building control-plane"

cd "$CONTROL_PLANE_DIR"
make build
log "Built: $CONTROL_PLANE_DIR/build/control-plane"

CP="$CONTROL_PLANE_DIR/build/control-plane"

# ─── Copy hello-world example (needed before credentials) ────────────────────

step "Setting up hello-world example"

EXAMPLE_DIR="$CONTROL_PLANE_DIR/my-first-sandbox"

if [[ -d "$EXAMPLE_DIR" ]]; then
    warn "$EXAMPLE_DIR already exists, skipping copy"
else
    cp -r "$CONTROL_PLANE_DIR/examples/hello-world" "$EXAMPLE_DIR"
    log "Copied hello-world example to $EXAMPLE_DIR"
fi

# ─── Store Anthropic API key ─────────────────────────────────────────────────

step "Setting up credentials"

ENV_FILE="$EXAMPLE_DIR/.env"

# Check if key is already in .env or will be set via env.
if [[ -f "$ENV_FILE" ]] && grep -q "anthropic_key=" "$ENV_FILE" 2>/dev/null; then
    log "anthropic_key already in .env, skipping"
elif [[ -n "${SECRET_ANTHROPIC_KEY:-}" ]]; then
    log "SECRET_ANTHROPIC_KEY already set, skipping"
else
    echo -e "${BOLD}Enter your Anthropic API key (starts with sk-ant-):${NC}"
    read -rsp "> " ANTHROPIC_KEY
    echo ""

    if [[ -z "$ANTHROPIC_KEY" ]]; then
        warn "No key entered. Add to .env or export SECRET_ANTHROPIC_KEY before running."
    else
        echo "anthropic_key=$ANTHROPIC_KEY" >> "$ENV_FILE"
        log "Added anthropic_key to $ENV_FILE"
    fi
fi

# ─── Done ─────────────────────────────────────────────────────────────────────

step "Setup complete"

echo -e "
${BOLD}What just happened:${NC}
  - Built GhostProxy, rootfs, and control-plane
  - Added Anthropic API key to my-first-sandbox/.env (or set SECRET_ANTHROPIC_KEY)
  - Created my-first-sandbox/ with a ready-to-run example

${BOLD}To run the hello-world example:${NC}

  ${CYAN}# Terminal 1: start GhostProxy${NC}
  $GHOSTPROXY_DIR/build/ghostproxy -addr :8090

  ${CYAN}# Terminal 2: boot the sandbox (uses .env by default)${NC}
  cd $EXAMPLE_DIR
  $CP up --name hello-world --secrets-provider env --secrets-dir .env

  ${CYAN}# When done:${NC}
  $CP status
  $CP down --id <container-id>

${BOLD}Or run it all at once:${NC}
  cd $EXAMPLE_DIR && ./run.sh
"
