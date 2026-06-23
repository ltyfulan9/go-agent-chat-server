#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
TOKEN="${TOKEN:?TOKEN is required}"
SESSION_ID="${SESSION_ID:?SESSION_ID is required}"
N="${N:-100}"
C="${C:-10}"
OUT_DIR="${OUT_DIR:-docs/bench-results}"

mkdir -p "$OUT_DIR"

if ! command -v hey >/dev/null 2>&1; then
  echo "hey not found. Install: go install github.com/rakyll/hey@latest" >&2
  exit 1
fi

BODY="$(mktemp)"
cat > "$BODY" <<JSON
{"session_id":"${SESSION_ID}","model":"mock","message":"benchmark mock llm"}
JSON

OUT="${OUT_DIR}/mock_chat_n${N}_c${C}.txt"
hey -n "$N" -c "$C" -m POST \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -D "$BODY" \
  "${BASE_URL}/api/chat" | tee "$OUT"

echo "saved: $OUT"
echo "mock model only measures backend chain + limiter overhead, not real Ollama/Coze generation throughput."
