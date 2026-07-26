#!/usr/bin/env bash
# Build the TypeScript snake into $1 (default: dist/).
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
out=${1:-$here/dist}

cd "$here"
npm ci
npm run build

if [ "$out" != "$here/dist" ]; then
  mkdir -p "$out"
  cp -r "$here/dist/." "$out/"
fi

echo "typescript -> $out"
