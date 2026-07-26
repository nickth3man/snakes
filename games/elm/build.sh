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

echo "elm -> $out ($(wc -c < "$out/elm.js") bytes of js)"
