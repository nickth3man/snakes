#!/usr/bin/env bash
# Build the C snake into $1 (default: dist/).
#
# No emscripten: clang targets wasm32 directly and wasm-ld links a module with
# no imports at all. Requires clang and lld.
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
out=${1:-$here/dist}

mkdir -p "$out"
clang \
  --target=wasm32 \
  -nostdlib \
  -O2 \
  -mbulk-memory \
  -Wall \
  -Wextra \
  -Wl,--no-entry \
  -Wl,--strip-all \
  -o "$out/snake.wasm" \
  "$here/snake.c"

cp "$here/index.html" "$out/"

# The demo AI is the same in every language, so every menu shows the same
# benchmark numbers, produced by `npm run benchmark` in games/typescript.
bench="$here/../typescript/public/benchmark.json"
if [ -f "$bench" ]; then cp "$bench" "$out/benchmark.json"; fi

echo "c -> $out ($(wc -c < "$out/snake.wasm") bytes of wasm)"
