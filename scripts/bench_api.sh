#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
TOKEN="${TOKEN:-}"
SESSION_ID="${SESSION_ID:-}"
N="${N:-1000}"
C="${C:-50}"
OUT_DIR="${OUT_DIR:-docs/bench-results}"

mkdir -p "$OUT_DIR"

if ! command -v hey >/dev/null 2>&1; then
  echo "hey not found. Install: go install github.com/rakyll/hey@latest" >&2
  exit 1
fi

AUTH_HEADER=()
if [[ -n "$TOKEN" ]]; then
  AUTH_HEADER=(-H "Authorization: Bearer ${TOKEN}")
fi

run_case() {
  local name="$1"
  local url="$2"
  local out="${OUT_DIR}/${name}_n${N}_c${C}.txt"
  echo "==> ${name}: ${url}"
  hey -n "$N" -c "$C" "${AUTH_HEADER[@]}" "$url" | tee "$out"
  echo "saved: $out"
}

run_case "sessions_page" "${BASE_URL}/api/sessions?page=1&page_size=20"

if [[ -n "$SESSION_ID" ]]; then
  run_case "messages_page" "${BASE_URL}/api/sessions/${SESSION_ID}/messages?page=1&page_size=30"
else
  echo "skip messages_page: set SESSION_ID to benchmark messages API" >&2
fi
