#!/usr/bin/env bash
set -euo pipefail

LLAMA_CPP_DIR="${LLAMA_CPP_DIR:-$HOME/src/llama.cpp}"
BUILD_DIR="${BUILD_DIR:-$LLAMA_CPP_DIR/build-opencompute}"

if [[ ! -f "$LLAMA_CPP_DIR/CMakeLists.txt" ]]; then
  echo "llama.cpp checkout not found: $LLAMA_CPP_DIR" >&2
  exit 1
fi

actual_commit="$(git -C "$LLAMA_CPP_DIR" rev-parse HEAD)"
expected_commit="2bc46baa9207d098d03741ac394e7faaf1a955b6"
if [[ "$actual_commit" != "$expected_commit" ]]; then
  echo "Unexpected llama.cpp commit: $actual_commit" >&2
  echo "Expected: $expected_commit" >&2
  exit 1
fi

echo "Building llama-server from $actual_commit"
cmake --build "$BUILD_DIR" --target llama-server --parallel 2

binary="$BUILD_DIR/bin/llama-server"
if [[ ! -x "$binary" ]]; then
  echo "Build completed but executable was not found: $binary" >&2
  exit 1
fi

"$binary" --version
echo "Built: $binary"
