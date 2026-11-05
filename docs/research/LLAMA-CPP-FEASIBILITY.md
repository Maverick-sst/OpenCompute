# llama.cpp Feasibility Notes for OpenCompute (M0)

**Research snapshot:** 2026-11-05  
**Upstream repository:** https://github.com/ggml-org/llama.cpp  
**Inspected upstream commit:** `2bc46baa9207d098d03741ac394e7faaf1a955b6`  
**Status:** Source inspection started; local build and runtime experiments still pending.

## 1. Why pin this snapshot

The upstream `master` ref resolved to commit `2bc46baa9207d098d03741ac394e7faaf1a955b6` during this inspection. This is a reproducible research snapshot, not an automatic endorsement for production. Before building, review the commit's security history and relevant RPC changes; use this exact SHA in the experiment checkout and do not build from a moving branch.

## 2. Confirmed from public headers and source at the inspected commit

### 2.1 Public model-loading and device-placement API exists

In `include/llama.h`:
- `llama_model_params` exposes a NULL-terminated `devices` list.
- It exposes `n_gpu_layers`, `split_mode`, `main_gpu`, and `tensor_split` fields (inspect the complete struct at the pinned SHA before coding).
- The split-mode enum includes `LLAMA_SPLIT_MODE_NONE`, `LLAMA_SPLIT_MODE_LAYER`, `LLAMA_SPLIT_MODE_ROW`, and `LLAMA_SPLIT_MODE_TENSOR`.
- The API includes `llama_model_load_from_file`, `llama_init_from_model`, `llama_decode`, and tokenization/logit accessors.
- The header exposes `llama_supports_rpc()`.

This suggests the GGML backend path is the lower-friction candidate for a first model-capacity experiment: the inference runtime has an API for choosing backend devices and split modes. It does **not** yet prove that a particular split mode across remote RPC devices works on our Linux host or that the model is split exactly as we intend.

### 2.2 GGML RPC is a backend interface, not a text-generation RPC

At the pinned SHA, `ggml/include/ggml-rpc.h` declares:
- `ggml_backend_rpc_init(endpoint, device)`
- `ggml_backend_rpc_buffer_type(endpoint, device)`
- `ggml_backend_rpc_get_device_memory(endpoint, device, ...)`
- `ggml_backend_rpc_start_server(endpoint, cache_dir, n_threads, n_devices, devices)`
- `ggml_backend_rpc_reg()` and `ggml_backend_rpc_add_server(endpoint)`

The RPC implementation serializes GGML tensor metadata including tensor type, dimensions, strides, operation, operation parameters, source tensor IDs and view references. Its command set includes buffer allocation/free, tensor set/get/copy, graph compute/recompute, device-memory query and collective communication commands.

The transport source documents this framing:
- request: command byte, request-size field, request bytes
- response: response-size field, response bytes

Therefore the remote endpoint is asked to perform backend-level tensor and graph operations. It is not simply passed a prompt string and asked to return generated text.

### 2.3 Weight caching is distinct from activation transfer

The inspected RPC source defines a 10 MiB hash-cache threshold. The exact predicate controlling whether a tensor uses hash caching must be verified at the pinned revision before relying on it. The previous PrivateLM code inspection found that its weight cache path is restricted to weight buffers; compute-buffer inputs/activations are not treated as reusable model weights.

For M0, instrument actual bytes transferred and cache hits/misses. Do not assume all model weights are sent once or that all activation traffic has a single fixed message per token.

### 2.4 Public llama API does not obviously expose arbitrary intermediate layer states

The public `llama.h` header exposes final output logits and optional embeddings. Those APIs are not documented as arbitrary hidden-state extraction/injection at a selected transformer-layer boundary.

The public model-loading parameters also do not expose a simple `layer_start/layer_end` range for loading a partial model slice.

This is a significant warning for the explicit layer-chain candidate:
- Do not equate `llama_get_embeddings_ith()` with a transformer hidden state at an arbitrary layer.
- Do not assume an ordinary GGUF can be loaded as a standalone partial-layer model.
- A true explicit layer-chain may require internal model/graph integration or a maintained llama.cpp fork.
- Establish the minimum required patch surface before choosing this path.

## 3. Initial hypothesis

**GGML RPC is the leading candidate for the first capacity proof**, because upstream has a public remote-backend API and device-splitting controls. The explicit layer-chain remains a valuable research candidate but currently has a higher apparent native-integration burden.

This is a provisional hypothesis, not the final ADR. The loopback build/run experiment must validate it.

## 4. Exact next source-inspection tasks

At the pinned SHA:
1. Read the full `llama_model_params` declaration and the implementation of model placement.
2. Trace how `tensor_split` and `split_mode` are consumed.
3. Trace `ggml_backend_rpc_add_server` registration and device enumeration.
4. Trace the RPC server entry point and its supported backend list.
5. Inspect model tensor placement for the selected model architecture.
6. Find whether any internal API or callback can expose intermediate hidden states without changing core model execution.
7. Inspect KV-cache allocation and whether its placement follows layer/device placement.
8. Review all security advisories and fixes applicable to the pinned commit.

## 5. Local Linux checks still required

Run these on the Linux host and save the output in `experiments/inference/m0/results/environment.txt`:

```bash
date -Is
uname -a
cat /etc/os-release
lscpu
free -h
df -h
cmake --version
ninja --version 2>/dev/null || true
gcc --version
g++ --version
git --version
```

Before compiling, confirm available disk and memory. The host previously discussed has approximately 12 GB RAM and 8 logical CPUs; two model processes plus build tools can exhaust memory. Use a small model, limit build parallelism (for example `-j2`), and avoid building several large targets concurrently.

## 6. Initial build/run experiment

1. Clone llama.cpp and checkout the exact pinned SHA.
2. Configure a CPU-only build with RPC enabled and tests/examples limited to what the experiment needs.
3. Build one inference executable and the RPC server target.
4. Run one local CPU inference and record model hash, prompt, output, load time, TTFT, decode speed and peak RSS.
5. Start one RPC server on loopback with a CPU backend.
6. Connect the master process to the RPC backend using the pinned public APIs / supported CLI options.
7. Confirm device enumeration and record model placement logs.
8. Add a second RPC server process and test two remote backend devices.
9. Compare memory use and correctness with the single-device reference.
10. Do not infer capacity success solely from a successful RPC handshake: verify actual remote allocation and execution.

The exact CMake target names and CLI flags must be confirmed from this pinned checkout rather than guessed.

## 7. Current conclusion

- **Confirmed:** upstream has a public GGML RPC backend API and model-device/split configuration.
- **Confirmed:** RPC serializes GGML tensor/graph metadata and supports remote graph computation.
- **Not yet confirmed:** the pinned build can distribute the selected model across two loopback RPC workers correctly.
- **Not yet confirmed:** model weights and KV cache placement provide the desired aggregate-memory capacity.
- **Not found in the inspected public API:** a simple arbitrary intermediate-layer activation extraction/injection interface or partial-layer model loader.
- **Pending:** local build, model choice, measurements, security audit and ADR.

Do not start a custom activation protocol or modify OpenCompute's coordinator until these M0 questions are answered.
