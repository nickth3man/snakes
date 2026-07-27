#!/usr/bin/env bash
# Build the Elm snake into $1 (default: dist/).
#
# The compiler is pulled in through npm so no extra toolchain setup is needed.
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
out=${1:-$here/dist}

mkdir -p "$out"
cd "$here"
npx --yes elm@0.19.2-0 make src/Main.elm --optimize --output="$out/elm.js"

cp "$here/index.html" "$out/"

# The demo AI is the same in every language, so every menu shows the same
# benchmark numbers, produced by `npm run benchmark` in games/typescript.
bench="$here/../typescript/public/benchmark.json"
if [ -f "$bench" ]; then cp "$bench" "$out/benchmark.json"; fi

echo "elm -> $out ($(wc -c < "$out/elm.js") bytes of js)"
