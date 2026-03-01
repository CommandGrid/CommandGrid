#!/usr/bin/env bash
# setup.sh — Bootstrap the agent sandbox system.
#
# Expects all three repos as sibling directories:
#   ../GhostProxy/
#   ../RootFS/
#   ../CommandGrid/   (this repo)
#
# What it does:
#   1. Clones missing sibling repos (GhostProxy, RootFS) into parent folder
#   2. Checks prerequisites (Go, Docker)
#   3. Builds GhostProxy
#   3. Builds the rootfs Docker image
#   4. Builds control-plane
#   5. Verifies credentials are configured (.env or Bitwarden)
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

# ─── Clone missing sibling repos ──────────────────────────────────────────────

step "Ensuring workspace layout"

# Derive org from this repo's remote (e.g. git@github.com:Travbz/control-plane.git -> Travbz)
ORIGIN_URL=""
if [[ -d "$SCRIPT_DIR/.git" ]]; then
    ORIGIN_URL=$(git -C "$SCRIPT_DIR" remote get-url origin 2>/dev/null || true)
fi
if [[ -z "$ORIGIN_URL" ]]; then
    warn "Not a git repo or no origin; cannot auto-clone. Clone GhostProxy and RootFS manually."
else
    # Support both git@ and https:// URLs
    ORG=""
    BASE=""
    if [[ "$ORIGIN_URL" =~ git@github\.com:([^/]+)/ ]]; then
        ORG="${BASH_REMATCH[1]}"
        BASE="git@github.com:$ORG"
    elif [[ "$ORIGIN_URL" =~ https?://[^/]*github\.com/([^/]+)/ ]]; then
        ORG="${BASH_REMATCH[1]}"
        BASE="https://github.com/$ORG"
    fi
    if [[ -n "$ORG" && -n "$BASE" ]]; then
        if [[ ! -d "$GHOSTPROXY_DIR" ]]; then
            log "Cloning GhostProxy (llm-proxy) into $GHOSTPROXY_DIR"
            git clone "$BASE/llm-proxy.git" "$GHOSTPROXY_DIR"
        else
            log "GhostProxy already present at $GHOSTPROXY_DIR"
        fi
        if [[ ! -d "$ROOTFS_DIR" ]]; then
            log "Cloning RootFS (sandbox-image) into $ROOTFS_DIR"
            git clone "$BASE/sandbox-image.git" "$ROOTFS_DIR"
        else
            log "RootFS already present at $ROOTFS_DIR"
        fi
    else
        warn "Could not parse org from origin; clone GhostProxy and RootFS manually."
    fi
fi

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

# ─── Verify credentials ──────────────────────────────────────────────────────

step "Verifying credentials"

ENV_FILE="$EXAMPLE_DIR/.env"
HAS_CREDS=false

# .env: must have anthropic_key= with a non-placeholder value
if [[ -f "$ENV_FILE" ]]; then
    VAL=$(grep -E "^anthropic_key=" "$ENV_FILE" 2>/dev/null | cut -d= -f2-)
    if [[ -n "$VAL" && "$VAL" != "sk-ant-..." && ${#VAL} -gt 20 ]]; then
        HAS_CREDS=true
        log "anthropic_key found in $ENV_FILE"
    fi
fi
# Or SECRET_ANTHROPIC_KEY env var
if [[ -n "${SECRET_ANTHROPIC_KEY:-}" && ${#SECRET_ANTHROPIC_KEY} -gt 20 ]]; then
    HAS_CREDS=true
    log "SECRET_ANTHROPIC_KEY is set"
fi
# Or Bitwarden has anthropic_key (bw must be unlocked)
if command -v bw &>/dev/null && [[ -n "${BW_SESSION:-}" ]]; then
    if bw list items --search anthropic_key --session "$BW_SESSION" 2>/dev/null | grep -q '"name":"anthropic_key"'; then
        HAS_CREDS=true
        log "anthropic_key found in Bitwarden"
    fi
fi

if [[ "$HAS_CREDS" != "true" ]]; then
    fail "anthropic_key not configured. Add it before running:

  Option 1 (env): Create $ENV_FILE with:
    anthropic_key=sk-ant-...

  Option 2 (env): Export before running:
    export SECRET_ANTHROPIC_KEY=\"sk-ant-...\"

  Option 3 (Bitwarden): Add a Login or Secure Note named 'anthropic_key'
    with your key, then run with --secrets-provider bitwarden

  See docs/secrets-local-dev.md for details."
fi

# ─── Done ─────────────────────────────────────────────────────────────────────

step "Setup complete"

echo -e "
${BOLD}What just happened:${NC}
  - Built GhostProxy, rootfs, and control-plane
  - Verified credentials (.env or SECRET_ANTHROPIC_KEY)
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
