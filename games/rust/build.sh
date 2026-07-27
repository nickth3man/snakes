#!/usr/bin/env bash
# Build the Rust snake into $1 (default: dist/).
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
out=${1:-$here/dist}

rustup target add wasm32-unknown-unknown
cargo build --release --target wasm32-unknown-unknown --manifest-path "$here/Cargo.toml"

mkdir -p "$out"
cp "$here/index.html" "$out/"
cp "$here/target/wasm32-unknown-unknown/release/snake_rs.wasm" "$out/snake.wasm"

# The demo AI is the same in every language, so every menu shows the same
# benchmark numbers, produced by `npm run benchmark` in games/typescript.
bench="$here/../typescript/public/benchmark.json"
if [ -f "$bench" ]; then cp "$bench" "$out/benchmark.json"; fi

echo "rust -> $out ($(wc -c < "$out/snake.wasm") bytes of wasm)"
