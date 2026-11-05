#!/usr/bin/env bash
set -euo pipefail

LLAMA_CPP_DIR="${LLAMA_CPP_DIR:-$HOME/src/llama.cpp}"
BUILD_DIR="${BUILD_DIR:-$LLAMA_CPP_DIR/build-opencompute}"
MODEL_PATH="${MODEL_PATH:-$HOME/models/opencompute/qwen2.5-0.5b-instruct-q4_k_m.gguf}"
HOST="${HOST:-127.0.0.1}"
PORT="${PORT:-8081}"
THREADS="${THREADS:-2}"
CTX_SIZE="${CTX_SIZE:-2048}"

SERVER="$BUILD_DIR/bin/llama-server"

if [[ ! -x "$SERVER" ]]; then
  echo "llama-server not found. Run experiments/inference/m1/build-llama-server.sh first." >&2
  exit 1
fi

if [[ ! -f "$MODEL_PATH" ]]; then
  echo "GGUF model not found: $MODEL_PATH" >&2
  exit 1
fi

if [[ "$HOST" != "127.0.0.1" && "$HOST" != "::1" && "$HOST" != "localhost" ]]; then
  echo "Refusing non-loopback bind during M1: $HOST" >&2
  exit 1
fi

echo "Starting local inference service"
echo "  model:   $MODEL_PATH"
echo "  address: $HOST:$PORT"
echo "  threads: $THREADS"
echo "  context: $CTX_SIZE"

exec "$SERVER" \
  --model "$MODEL_PATH" \
  --host "$HOST" \
  --port "$PORT" \
  --threads "$THREADS" \
  --threads-batch "$THREADS" \
  --ctx-size "$CTX_SIZE" \
  --n-gpu-layers 0
