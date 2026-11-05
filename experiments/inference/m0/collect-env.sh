#!/usr/bin/env bash
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
OUT_DIR="$ROOT/experiments/inference/m0/results"
mkdir -p "$OUT_DIR"
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
OUT="$OUT_DIR/environment-$STAMP.txt"

run() {
  local label="$1"
  shift
  {
    printf '\n===== %s =====\n' "$label"
    if command -v "$1" >/dev/null 2>&1; then
      "$@" 2>&1 || true
    else
      printf 'not installed: %s\n' "$1"
    fi
  } >>"$OUT"
}

{
  echo "OpenCompute M0 environment manifest"
  echo "captured_at_utc=$(date -u -Is)"
  echo "working_directory=$ROOT"
  echo "NOTE: no environment variables, credentials, tokens, or SSH keys are collected."
} >"$OUT"

run "Kernel and OS" uname -a
run "OS release" cat /etc/os-release
run "CPU topology" lscpu
run "Memory" free -h
run "Filesystem capacity" df -h
run "CMake" cmake --version
run "Ninja" ninja --version
run "GCC" gcc --version
run "G++" g++ --version
run "Clang" clang --version
run "Git" git --version
run "Docker" docker --version
run "Docker Compose" docker compose version
run "NVIDIA driver/runtime (optional)" nvidia-smi
run "Vulkan summary (optional)" vulkaninfo --summary

{
  printf '\n===== Cgroup memory limits (if available) =====\n'
  for f in /sys/fs/cgroup/memory.max /sys/fs/cgroup/memory.current; do
    if [ -r "$f" ]; then printf '%s=' "$f"; cat "$f"; fi
  done
  printf '\n===== CPU quota (if available) =====\n'
  for f in /sys/fs/cgroup/cpu.max; do
    if [ -r "$f" ]; then printf '%s=' "$f"; cat "$f"; fi
  done
} >>"$OUT"

echo "Wrote: $OUT"
