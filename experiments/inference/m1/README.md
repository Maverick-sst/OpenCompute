# M1 — Persistent native inference service

M1 starts with a persistent single-model inference process. It is deliberately separate from the Go coordinator: first prove that the native runtime can stay loaded, serve repeated requests, expose health, and shut down cleanly. Coordinator registration and scheduling come later.

## Current lab assumptions

- llama.cpp checkout: `~/src/llama.cpp`
- pinned runtime commit: `2bc46baa9207d098d03741ac394e7faaf1a955b6`
- build directory: `build-opencompute`
- smoke-test model: `~/models/opencompute/qwen2.5-0.5b-instruct-q4_k_m.gguf`
- bind address: `127.0.0.1` only
- port: `8081`
- CPU threads: 2 (conservative on the 4-core host)

## 1. Build llama-server

From the OpenCompute repository root:

```bash
bash experiments/inference/m1/build-llama-server.sh
```

This builds the upstream `llama-server` target in the existing pinned checkout with at most two parallel jobs. It does not modify or vendor llama.cpp into this repository.

## 2. Start the persistent inference service

```bash
bash experiments/inference/m1/run-llama-server.sh
```

The server loads the GGUF model once and remains running. It must bind only to loopback during this milestone.

## 3. Verify health

In another terminal:

```bash
curl --fail --silent --show-error http://127.0.0.1:8081/health
```

## 4. Send two sequential requests

```bash
curl --fail --silent --show-error http://127.0.0.1:8081/completion \
  -H 'Content-Type: application/json' \
  -d '{"prompt":"Explain distributed computing in simple terms.","n_predict":64,"temperature":0}'
```

Run the same request again. The purpose is to establish repeated inference without reloading the model between requests.

## 5. Record service behaviour

Capture:
- server startup/model-load output;
- health response;
- two inference responses and timings;
- process RSS while idle and during inference;
- graceful shutdown using Ctrl+C.

## Security and scope

- Do not bind to `0.0.0.0`, a LAN interface, or a public address.
- Do not expose the llama.cpp server directly to the internet.
- This is a local trusted-lab experiment with no authentication.
- Do not put activation payloads or model execution through the Go coordinator.
- This milestone does not yet integrate the existing synthetic worker protocol.
- A successful HTTP inference test proves a persistent native inference service, not model-parallel execution.

## M1 acceptance for this first slice

- [ ] `llama-server` builds from the pinned runtime.
- [ ] Server starts and loads the selected model once.
- [ ] `/health` reports healthy.
- [ ] Two sequential inference requests succeed.
- [ ] Server shuts down cleanly.
- [ ] Startup, request, memory and timing evidence is recorded.
