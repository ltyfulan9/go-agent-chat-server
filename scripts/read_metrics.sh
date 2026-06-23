#!/usr/bin/env bash
set -euo pipefail

DEBUG_URL="${DEBUG_URL:-http://127.0.0.1:6060}"

curl -s "${DEBUG_URL}/debug/vars" | python3 -m json.tool
