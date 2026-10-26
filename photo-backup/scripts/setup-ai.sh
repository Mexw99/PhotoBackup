#!/usr/bin/env bash
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

if [[ -n "${PYTHON:-}" ]]; then
  PYTHON_BIN="$PYTHON"
elif command -v python3 >/dev/null 2>&1; then
  PYTHON_BIN="python3"
elif command -v python >/dev/null 2>&1; then
  PYTHON_BIN="python"
else
  echo "ไม่พบ Python 3 กรุณาติดตั้ง Python 3 ก่อน" >&2
  exit 1
fi

VENV="$PROJECT_ROOT/.venv-ai"
"$PYTHON_BIN" -m venv "$VENV"
"$VENV/bin/python" -m pip install --upgrade pip
"$VENV/bin/python" -m pip install -r requirements-ai.txt

echo "ติดตั้ง Local AI สำเร็จ"
echo "ใช้ Python: $VENV/bin/python"
echo "จากนั้นรัน: wails dev"
