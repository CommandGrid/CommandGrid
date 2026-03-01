#!/usr/bin/env bash
set -euo pipefail

echo "=== Hello Weather Workflow (Proxy Mode) ==="
echo "ANTHROPIC_API_KEY set: $([ -n "${ANTHROPIC_API_KEY:-}" ] && echo yes || echo NO)"
echo "ANTHROPIC_BASE_URL: ${ANTHROPIC_BASE_URL:-not set}"
echo ""

if [[ -z "${ANTHROPIC_API_KEY:-}" || -z "${ANTHROPIC_BASE_URL:-}" ]]; then
  echo "ERROR: proxy secrets were not injected correctly."
  exit 1
fi

if [[ ! -x /workspace/workflow-hello-weather ]]; then
  echo "ERROR: /workspace/workflow-hello-weather is missing or not executable."
  exit 1
fi

cat <<'JSON' | /workspace/workflow-hello-weather
{
  "task_id": "hello-weather-proxy",
  "prompt": "Provide a concise 5-day weather summary with practical recommendations.",
  "location": "Montrose",
  "state": "Colorado",
  "country": "US",
  "days": 5
}
JSON
