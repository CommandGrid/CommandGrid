#!/usr/bin/env bash
# agent.sh — Hello world agent.
#
# Makes a single Anthropic API call using curl to prove the proxy flow works.
# The ANTHROPIC_API_KEY env var holds a session token (not the real key).
# The ANTHROPIC_BASE_URL env var points at the llm-proxy.
# The proxy swaps the session token for the real key before forwarding upstream.

set -euo pipefail

echo "=== Hello World Agent ==="
echo ""
echo "Checking environment..."
echo "  ANTHROPIC_API_KEY is set: $([ -n "${ANTHROPIC_API_KEY:-}" ] && echo 'yes' || echo 'NO')"
echo "  ANTHROPIC_BASE_URL: ${ANTHROPIC_BASE_URL:-not set}"
echo ""

if [[ -z "${ANTHROPIC_API_KEY:-}" ]]; then
    echo "ERROR: ANTHROPIC_API_KEY is not set. Something went wrong with secret injection."
    exit 1
fi

if [[ -z "${ANTHROPIC_BASE_URL:-}" ]]; then
    echo "ERROR: ANTHROPIC_BASE_URL is not set. The proxy base URL was not injected."
    exit 1
fi

echo "Making API call through the proxy..."
echo "  Target: ${ANTHROPIC_BASE_URL}/v1/messages"
echo ""

RESPONSE=$(curl -sf "${ANTHROPIC_BASE_URL}/v1/messages" \
    -H "x-api-key: ${ANTHROPIC_API_KEY}" \
    -H "content-type: application/json" \
    -H "anthropic-version: 2023-06-01" \
    -d '{
        "model": "claude-sonnet-4-20250514",
        "max_tokens": 128,
        "messages": [
            {"role": "user", "content": "Say hello world and nothing else."}
        ]
    }' 2>&1) || {
    echo "API call failed. Response:"
    echo "$RESPONSE"
    echo ""
    echo "If you see 'invalid session token', the proxy session was not registered."
    echo "If you see 'upstream request failed', the proxy is working but the real key may be wrong."
    exit 1
}

echo "Response from Claude (via proxy):"
echo "$RESPONSE" | python3 -c "
import sys, json
try:
    data = json.load(sys.stdin)
    for block in data.get('content', []):
        if block.get('type') == 'text':
            print('  ' + block['text'])
except:
    print(sys.stdin.read())
" 2>/dev/null || echo "$RESPONSE"

echo ""
echo "=== Done ==="
echo "The API call went: sandbox -> llm-proxy -> Anthropic"
echo "Your real API key never entered this container."
