#!/usr/bin/env sh
# shellcheck shell=sh
set -eu
DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$DIR/lib.sh"
ROOT=$(repo_root)
REPORTS="$ROOT/tests/reports"
GO_JSON="$REPORTS/backend-go-test.json"

pass=0; fail=0
if [ -f "$GO_JSON" ]; then
  # grep -c 在零匹配时打印 0 并以状态 1 退出；用 || true 吞掉退出码，保留计数。
  pass=$(grep -c '"Action":"pass"' "$GO_JSON" || true)
  fail=$(grep -c '"Action":"fail"' "$GO_JSON" || true)
fi

ts=$(date -u +%Y-%m-%dT%H:%M:%SZ)
printf '{"generatedAt":"%s","backend":{"pass":%s,"fail":%s}}\n' "$ts" "$pass" "$fail" > "$REPORTS/summary.json"
{
  echo "# 回归汇总 ($ts)"
  echo
  echo "## 后端 (go test)"
  echo "- pass: $pass"
  echo "- fail: $fail"
  echo
  echo "## 前端 (Playwright)"
  echo "- HTML 报告: tests/reports/playwright-html/index.html"
  echo "- 结果 JSON: tests/reports/playwright-results.json"
  echo "- 截图: tests/frontend/screenshots/"
} > "$REPORTS/summary.md"

log "summary written: $REPORTS/summary.md"
[ "$fail" -eq 0 ]
