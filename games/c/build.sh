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

echo "c -> $out ($(wc -c < "$out/snake.wasm") bytes of wasm)"
