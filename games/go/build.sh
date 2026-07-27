#!/usr/bin/env bash
# Build the Go snake into $1 (default: dist/).
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
out=${1:-$here/dist}

mkdir -p "$out"
cd "$here"
GOOS=js GOARCH=wasm go build -trimpath -ldflags="-s -w" -o "$out/snake.wasm" .

# The JS shim that boots the Go runtime ships with the toolchain. Go 1.24 moved
# it from misc/wasm to lib/wasm.
goroot=$(go env GOROOT)
for candidate in "$goroot/lib/wasm/wasm_exec.js" "$goroot/misc/wasm/wasm_exec.js"; do
  if [ -f "$candidate" ]; then
    cp "$candidate" "$out/wasm_exec.js"
    break
  fi
done
if [ ! -f "$out/wasm_exec.js" ]; then
  echo "could not find wasm_exec.js under $goroot" >&2
  exit 1
fi

cp "$here/index.html" "$out/"

# The demo AI is the same in every language, so every menu shows the same
# benchmark numbers, produced by `npm run benchmark` in games/typescript.
bench="$here/../typescript/public/benchmark.json"
if [ -f "$bench" ]; then cp "$bench" "$out/benchmark.json"; fi

echo "go -> $out ($(wc -c < "$out/snake.wasm") bytes of wasm)"
