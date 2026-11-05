# M0 — Native inference feasibility

This directory contains the first reproducible OpenCompute inference experiment. M0 is a feasibility gate, not production integration.

## Locked lab decisions

- Linux host.
- Two isolated worker processes for the eventual loopback experiment.
- Compare GGML RPC and explicit layer-chain feasibility.
- Capacity is the first model-parallel objective.
- CPU backend first; no accelerator assumption.

## Step 1: collect the host manifest

From the OpenCompute repository root:

```bash
bash experiments/inference/m0/collect-env.sh
```

The script writes a timestamped environment report under `experiments/inference/m0/results/`. Review it before sharing. It captures hardware/software inventory and disk/memory availability; it does not read credentials or environment variables.

## Step 2: pin the native runtime

M0 research snapshot:

- Repository: https://github.com/ggml-org/llama.cpp
- Commit: `2bc46baa9207d098d03741ac394e7faaf1a955b6`

Use a separate checkout outside the OpenCompute repository:

```bash
mkdir -p ~/src
git clone https://github.com/ggml-org/llama.cpp.git ~/src/llama.cpp
cd ~/src/llama.cpp
git checkout 2bc46baa9207d098d03741ac394e7faaf1a955b6
git rev-parse HEAD
git status --short
```

Do not use a moving `master` checkout for benchmark results. Keep llama.cpp separate from OpenCompute; do not vendor it yet.

## Step 3: stop before building if resources are tight

The previously reported Linux host has approximately 12 GB RAM and 8 logical CPUs. Build with a low parallelism limit (start with `-j2`), avoid building during heavy workloads, and use a small model. The exact available RAM/disk must come from the new environment report.

## Step 4: return the environment report

Share the generated report or paste its contents. Do not share API tokens, SSH keys, model-provider credentials or unrelated environment variables.

## M0 exit checklist

- [ ] Host environment recorded.
- [ ] llama.cpp checkout pinned to the exact commit.
- [ ] CPU-only single-device inference smoke test passes.
- [ ] RPC server build and loopback handshake pass.
- [ ] Remote backend device enumeration and actual remote tensor allocation/execution verified.
- [ ] Two-worker capacity behaviour measured or marked unsupported.
- [ ] Explicit layer-chain API/patch feasibility documented.
- [ ] Security review completed for the exact pinned revision.
- [ ] ADR-001 records the chosen path and evidence.

No coordinator, worker protocol or dashboard changes should be made until the feasibility results are reviewed.
