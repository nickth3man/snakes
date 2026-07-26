#!/usr/bin/env bash
# "Build" the Python snake into $1 (default: dist/).
#
# There is nothing to compile: Pyodide fetches main.py and runs it in the
# browser, so this only syntax-checks the source and stages the two files.
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
out=${1:-$here/dist}

py=$(command -v python3 || command -v python)
"$py" - "$here/main.py" <<'CHECK'
import ast, sys
ast.parse(open(sys.argv[1], encoding="utf-8").read(), sys.argv[1])
CHECK

mkdir -p "$out"
cp "$here/index.html" "$here/main.py" "$out/"

echo "python -> $out"
