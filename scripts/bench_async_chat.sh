#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
TOKEN="${TOKEN:?TOKEN is required}"
SESSION_ID="${SESSION_ID:?SESSION_ID is required}"
N="${N:-200}"
C="${C:-50}"
OUT_DIR="${OUT_DIR:-docs/bench-results}"

mkdir -p "$OUT_DIR"

if ! command -v hey >/dev/null 2>&1; then
  echo "hey not found. Install: go install github.com/rakyll/hey@latest" >&2
  exit 1
fi

BODY="$(mktemp)"
cat > "$BODY" <<JSON
{"session_id":"${SESSION_ID}","model":"mock","message":"benchmark async llm"}
JSON

OUT="${OUT_DIR}/async_chat_submit_n${N}_c${C}.txt"
hey -n "$N" -c "$C" -m POST \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -D "$BODY" \
  "${BASE_URL}/api/chat/async" | tee "$OUT"

echo "saved: $OUT"
echo "This measures async job submission throughput. Worker throughput is observed through /debug/vars: chat_job_success_total, chat_job_queue_wait_ms_total, llm_call_total."
