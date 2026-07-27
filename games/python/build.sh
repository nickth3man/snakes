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

# The demo AI is the same in every language, so every menu shows the same
# benchmark numbers, produced by `npm run benchmark` in games/typescript.
bench="$here/../typescript/public/benchmark.json"
if [ -f "$bench" ]; then cp "$bench" "$out/benchmark.json"; fi

echo "python -> $out"
