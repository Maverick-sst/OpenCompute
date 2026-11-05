# OpenCompute Distributed Inference: Research and Experiment Specification

**Status:** Proposed / implementation baseline  
**Decision date:** 2026-11-05  
**Scope:** Research, feasibility, and controlled experiments before production implementation.

## 1. Locked decisions

| Decision | Locked choice |
|---|---|
| Native execution research | Prototype both GGML RPC and an explicit activation/layer-chain path |
| First model-parallel objective | Capacity first: run a model that does not fit on one worker, even if latency is worse |
| Initial lab | One Linux host, two isolated worker processes |
| Control plane | Existing OpenCompute Go coordinator |
| Native inference runtime | llama.cpp / GGML as the first candidate |
| Initial backend | CPU |
| Initial model format | GGUF |
| Initial network | Loopback; move to LAN only after correctness |
| Security | Local trusted lab only; no public RPC exposure |

These choices do not assert that either native path is already proven. Both are hypotheses to test.

## 2. Existing OpenCompute baseline

The repository already contains:
- Go coordinator, worker registry, heartbeats, leases, fencing tokens and synthetic range jobs.
- JSON file-backed coordinator state for a single coordinator process.
- JSONL dataset upload, SHA-256 metadata and deterministic newline-aligned partition manifests.
- A streaming log analytics processor and partial aggregate merge helpers.
- React dashboard with JSONL upload preview.

The existing Go worker still executes a synthetic integer-range workload. It does not load a model, hold a KV cache, transfer activations or perform inference. Dataset ingestion is a separate data-plane foundation and is not the inference runtime.

Keep V0 synthetic jobs and the V1 dataset workflow backward-compatible. Do not overload dataset partitions or integer-range batches to represent model-layer partitions.

## 3. Research questions and architecture candidates

### Candidate A: GGML RPC / remote backend

Use llama.cpp/GGML RPC to expose remote GGML devices to the master inference runtime. Investigate:
- Exact runtime revision and RPC security fixes.
- Remote backend registration and device discovery.
- Tensor split and model placement semantics.
- Which buffers and operations execute remotely.
- Weight caching versus live activation transfer.
- Whether the selected runtime build supports the target CPU backend and later accelerators.
- Failure behaviour when a remote endpoint disappears during graph execution.

Advantages: reuse native graph execution and backend abstractions. Risks: runtime coupling, RPC attack surface, less application-level control over individual layer boundaries, and uncertain failure recovery.

### Candidate B: explicit layer-chain / activation protocol

Each native worker loads a contiguous model-layer range. A preceding worker sends a hidden-state activation to the next worker. The final stage executes the output head and returns logits/token output.

Investigate:
- Whether upstream llama.cpp can load only selected layer ranges without invasive patches.
- Whether the C/C++ API exposes a safe point to extract and inject hidden states.
- Whether partial GGUF loading requires a maintained fork or custom loader.
- How tokenizer, embeddings, output head, RoPE, normalization and architecture-specific details are assigned.
- How per-worker KV cache is retained and addressed by session.
- Tensor serialization, ordering, backpressure and retry semantics.

Advantages: explicit placement, observable stage boundaries and application-owned recovery. Risks: substantial native-runtime work and architecture-specific model handling.

### Comparison protocol

Build a minimal feasibility spike for both candidates before choosing a production execution path. Compare:
- Correctness against single-worker reference output.
- Whether the model can actually be split across two processes.
- Model load and memory footprint per worker.
- Activation bytes per boundary and per decode step.
- TTFT, decode tokens/sec and end-to-end latency.
- Complexity of session/KV-cache management.
- Behaviour on worker loss.
- Security and version-maintenance burden.

Do not choose based on code elegance or README claims alone.

## 4. Target architecture

### Control plane (Go)

Owns worker identity, capability inventory, heartbeats, model metadata, placement plans, session creation, worker leases, lifecycle, retries, fencing and telemetry aggregation.

The coordinator must not proxy activation payloads in the inference hot path.

### Data plane (native workers)

A persistent native worker process owns model loading, model partition validation, inference backend, KV-cache allocation, tensor execution, activation serialization/transport and execution metrics.

A lightweight supervisor/agent registers the worker and reports lifecycle/capabilities to the Go coordinator. The first prototype may keep the supervisor and native process in one local wrapper, but their responsibilities should remain logically separate.

### Transport

Use gRPC initially for worker control and the first activation prototype. Benchmark it. Keep transport behind an interface so a framed TCP or other binary transport can be compared later. Do not prematurely optimize transport before measuring payload sizes and latency.

### Session placement

An inference session is pinned to its worker assignments while its KV cache is resident. A worker is not interchangeable with another merely because it has the same model. Reassignment requires an explicit recovery operation: initially replay the prompt and generated token history; later evaluate KV-cache checkpoint/transfer.

## 5. Initial conceptual protocol

Control messages:
- RegisterWorker
- Heartbeat
- AdvertiseCapabilities
- AdvertiseModel
- CreateSession
- AssignModelPartition
- StartInference
- StopInference
- ReportMetrics
- ReportSessionStatus

Activation envelope (illustrative; not yet frozen):
- protocol_version
- session_id
- request_id
- step_id / sequence_number
- model_id and immutable model_revision/hash
- partition_id and layer_start/layer_end
- tensor dimensions, dtype, layout and byte length
- payload checksum
- opaque tensor payload

Session and worker identity:
- worker_id
- lease_id
- fencing_token
- session epoch
- request idempotency key

The tensor payload should be bounded and validated before allocation. The protocol must reject stale, duplicate, out-of-order, wrong-model, wrong-partition and incompatible-shape messages. The final wire schema must be based on the native runtime's actual tensor layout, not guessed in advance.

## 6. Resource and scheduling model

A worker capability record should include:
- CPU architecture/model, logical and available cores.
- Total and available RAM.
- RAM bandwidth estimate (clearly distinguished from network bandwidth).
- Accelerator backend, device memory and supported operations.
- Runtime build/version and supported model architectures.
- Local model artifacts and hashes.
- Current load, active sessions and memory pressure.
- Pairwise network RTT, effective throughput and recent failure rate.
- Thermal/power status where the platform exposes it.

Initial placement heuristic:
1. Reject workers that cannot load the assigned model slice and reserve KV-cache headroom.
2. Reject incompatible runtime, architecture, tensor dtype or model revision.
3. Estimate stage compute time from measured workload data.
4. Estimate communication time as latency plus payload_size / effective_bandwidth.
5. Account for queueing and model initialization.
6. Choose a feasible placement minimizing estimated end-to-end time, subject to the capacity objective.

Do not implement a learned scheduler until benchmark data exists. Equal layer counts are not assumed to mean equal stage times.

## 7. Experiment plan

### E0 — single-device reference
- One worker process and one complete model.
- Pin model hash, quantization, runtime revision, prompt, context, decoding settings and generated token count.
- Record load time, TTFT, decode tokens/sec, peak RSS, CPU, accelerator utilization, KV-cache memory and output.
- Warm up; run at least 5 measured repetitions; report median and dispersion.

### E1 — request-level distribution
- Two independent workers, each with a complete model only if memory allows.
- Send independent requests through the coordinator.
- Measure aggregate requests/sec and tokens/sec, per-request TTFT/completion time, queue wait, utilization and failures.
- Expect aggregate throughput gains only with enough independent requests; do not expect one request to become faster.

### E2 — two-worker model-parallel correctness and capacity
- Two isolated worker processes on the same Linux host.
- Compare single-device, GGML RPC and explicit layer-chain paths.
- First prove correct output and that each process holds only its assigned model resources.
- Prefer a model that cannot fit in one worker's assigned memory budget but can fit in the combined budget. Do not force this on the initial 12 GB host if the model/runtime overhead makes it unsafe; first use a small model to prove mechanics.
- Record per-worker model memory, KV-cache memory, activation bytes, transfer time, TTFT, decode tokens/sec and output correctness.
- Capacity success is valid even if latency is worse.

### E3 — network sensitivity
- Hold model, prompt and worker hardware constant.
- Compare loopback, LAN/Ethernet and Wi-Fi when available.
- Introduce controlled latency, bandwidth limits and packet loss.
- Record RTT, effective throughput, activation payload size, transfer time and inference latency.
- Identify the break-even region; do not extrapolate beyond measured conditions.

### E4 — placement balance
- Test multiple layer splits (for example 50/50, 60/40, 70/30 and reverse splits) only where the runtime supports them.
- Record stage compute time, idle/wait time, transfer time, memory and overall decode latency.
- Compare measured stage times, not layer counts alone.

### E5 — failure and replay
- Kill a worker before session start, during prefill and during decode.
- Test timeout, disconnect/reconnect, duplicate activation, stale lease/fencing token and out-of-order step.
- Initial recovery: invalidate the old session epoch and replay prompt plus generated token history on a replacement placement.
- Verify no duplicate token acceptance or corrupted session state.

### E6 — context/model scaling
- Increase model size and context length in controlled steps.
- Record weight memory, KV-cache growth, activation dimensions/bytes, peak RSS, load time, TTFT and decode speed.
- Establish capacity envelope per worker and cluster.

## 8. Benchmark record

Store one JSON record per run, for example:

```json
{
  "experiment_id": "E2-run-001",
  "timestamp": "RFC3339",
  "git_commit": "sha",
  "runtime_commit": "pinned llama.cpp commit",
  "model_sha256": "sha256",
  "model_architecture": "architecture",
  "quantization": "quant",
  "strategy": "single|request-parallel|ggml-rpc|layer-chain",
  "worker_count": 2,
  "layer_ranges": [[0, 13], [14, 27]],
  "network_profile": "loopback",
  "prompt_tokens": 256,
  "generated_tokens": 128,
  "context_length": 512,
  "load_time_ms": 0,
  "ttft_ms": 0,
  "decode_tokens_per_second": 0,
  "end_to_end_ms": 0,
  "activation_bytes_total": 0,
  "activation_bytes_per_decode_step": 0,
  "transfer_time_ms_total": 0,
  "worker_peak_rss_bytes": [0, 0],
  "worker_cpu_percent": [0, 0],
  "kv_cache_bytes": [0, 0],
  "output_match": true,
  "failure": null
}
```

All zero values above are placeholders. Record raw per-run data; calculate median, p95 and variation in analysis. Separate prefill and decode metrics where the runtime permits. Report model-load time separately from steady-state inference.

## 9. Correctness and acceptance criteria

A run is valid only if:
- Exact model artifact and runtime revision are recorded.
- Input and decoding parameters match the reference.
- All tensor dimensions, dtypes and layer ranges are validated.
- Output is valid and matches the reference under deterministic settings, or meets a documented numerical/token-equivalence tolerance where execution order or kernels differ.
- No worker reports an unhandled error or memory overrun.
- Activation checksums and step ordering are verified.
- Metrics are complete enough to reproduce the run.

Milestone E2 passes when one model executes across two worker processes, the worker ownership of model partitions is demonstrated, output correctness passes, and capacity/memory measurements are captured. It does not require a speedup.

## 10. Security and reliability guardrails

- Localhost or isolated trusted LAN only during these experiments.
- Never expose unauthenticated GGML RPC to the public internet.
- Pin and audit the exact llama.cpp revision and RPC implementation.
- Use authenticated worker identity and encrypted transport before cross-trust-boundary deployment.
- Validate message size, tensor dimensions, dtype, model revision, layer range and session ownership.
- Use fencing tokens/session epochs to reject stale workers.
- Do not trust worker self-reported benchmark data for security-sensitive decisions.
- Apply memory and request limits; never load arbitrary model files from untrusted workers.
- Treat worker loss as a session failure until recovery is proven; a heartbeat is not proof of healthy inference.

## 11. Milestone roadmap and gates

### M0 — native-runtime feasibility (no production integration)
Tasks:
1. Record Linux host OS/kernel, CPU topology, RAM, available disk, compiler and CMake versions.
2. Select and pin a llama.cpp commit; record submodules and build options.
3. Build a single-device CPU inference smoke test using a small, licensed GGUF model.
4. Identify public C/C++ APIs for model loading, context creation, tokenization, prefill and decode.
5. Trace how model layers, output head and KV cache are allocated.
6. Determine whether upstream supports remote GGML RPC in the chosen build and document exact activation/data movement.
7. Determine whether the runtime supports partial model loading and hidden-state injection/extraction without a fork.
8. Build a minimal two-process GGML RPC proof on loopback.
9. Prototype the feasibility of an explicit activation boundary; do not implement a complete custom model loader unless upstream interfaces prove it necessary.
10. Compare results and issue an architecture decision record (ADR).

M0 deliverables:
- `docs/research/LLAMA-CPP-FEASIBILITY.md`
- `docs/adr/ADR-001-inference-execution-path.md`
- Reproducible build/run scripts under `experiments/inference/m0/`
- Raw benchmark outputs and environment manifest.
- A written go/no-go decision for each candidate.

M0 exit gate:
- Single-device inference works reproducibly.
- Exact runtime commit/build configuration is pinned.
- GGML RPC path is understood and tested on loopback or explicitly marked blocked.
- Explicit layer-chain feasibility is evidenced by API/source inspection or a minimal native spike.
- We know whether custom llama.cpp patches are required.
- No public RPC exposure.
- ADR selects the next implementation path based on evidence, while preserving the second path as a comparison if useful.

### M1 — native worker
Persistent C++ worker, model loading, capability report, inference API, graceful shutdown and metrics.

### M2 — OpenCompute request-level inference
Register inference workers, advertise model/runtime capabilities, create sessions and distribute independent requests.

### M3 — two-worker model parallelism
Model partition metadata, activation transport, per-worker KV cache, session routing and correctness against single-device reference.

### M4 — scheduling and resilience
Measured capability-aware placement, network-aware estimates, fencing, replay recovery and failure tests.

### M5 — heterogeneous devices
Add Mac/Linux and then Android feasibility, accelerator backends, thermal/power telemetry and placement comparisons.

## 12. Proposed repository layout

```text
docs/
  research/
    INFERENCE-RESEARCH-AND-EXPERIMENTS.md
    LLAMA-CPP-FEASIBILITY.md
  adr/
    ADR-001-inference-execution-path.md
experiments/
  inference/
    m0/
      README.md
      env.sh
      build-llama.sh
      smoke-test.sh
      ggml-rpc/
      layer-chain/
      results/
native/
  inference-worker/       # introduced after M0 decision
proto/
  inference/v1/           # protocol frozen after native feasibility
internal/
  inference/              # Go control-plane/session metadata later
```

This layout is a proposal; do not create empty scaffolding before the corresponding milestone needs it.

## 13. Explicit non-goals for M0

- No production scheduler.
- No dashboard changes.
- No Android worker implementation.
- No tensor-parallel/all-reduce implementation.
- No public networking or untrusted worker participation.
- No custom GGUF format or model conversion.
- No claim of speedup.
- No changes to the existing V0 job protocol or V1 JSONL dataset workflow.

## 14. Immediate next actions

1. Capture the Linux host's actual hardware/software environment.
2. Inspect the current llama.cpp RPC implementation and version/security guidance.
3. Build the smallest reproducible CPU inference baseline.
4. Run the loopback GGML RPC feasibility test.
5. Investigate explicit hidden-state/layer-slice feasibility.
6. Write the ADR with measured evidence before committing to M1.

**Decision rule:** capacity is the first model-parallel objective. A slower distributed run is still a successful experiment if it proves correct execution of a model that cannot fit on a single worker under the same memory budget. Speed is a separate measured outcome.
