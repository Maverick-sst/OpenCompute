# M1 — Persistent native inference service

M1 starts with a persistent single-model inference process. It is deliberately separate from the Go coordinator: first prove that the native runtime can stay loaded, serve repeated requests, expose health, and shut down cleanly. Coordinator registration and scheduling come later.

## Current lab assumptions

- llama.cpp checkout: `~/src/llama.cpp`
- pinned runtime commit: `2bc46baa9207d098d03741ac394e7faaf1a955b6`
- build directory: `build-opencompute`
- smoke-test model: `~/models/opencompute/qwen2.5-0.5b-instruct-q4_k_m.gguf`
- native inference bind: `127.0.0.1:8081`
- supervisor health bind: `127.0.0.1:8090`
- CPU threads: 2 (conservative on the 4-core host)

## 1. Build llama-server

From the OpenCompute repository root:

```bash
bash experiments/inference/m1/build-llama-server.sh
```

This builds the upstream `llama-server` target in the existing pinned checkout with at most two parallel jobs. It does not modify or vendor llama.cpp into this repository.

## 2. Run the native server directly (smoke-test path)

```bash
bash experiments/inference/m1/run-llama-server.sh
```

The server loads the GGUF model once and remains running. It must bind only to loopback during this milestone.

## 3. Run the Go supervisor

Stop the directly launched llama-server first with Ctrl+C. Then, from the OpenCompute repository root:

```bash
gofmt -w internal/inference/supervisor.go internal/inference/supervisor_test.go cmd/inference-worker/main.go
go test ./...
go run ./cmd/inference-worker
```

The Go process starts llama-server as a child, waits for its `/health` endpoint to report `{"status":"ok"}`, and exposes its own status at `http://127.0.0.1:8090/healthz`. It forwards child stdout/stderr to the terminal and shuts the child down on Ctrl+C/SIGTERM.

Optional overrides:

```bash
go run ./cmd/inference-worker \
  -llama-server "$HOME/src/llama.cpp/build-opencompute/bin/llama-server" \
  -model "$HOME/models/opencompute/qwen2.5-0.5b-instruct-q4_k_m.gguf" \
  -native-host 127.0.0.1 -native-port 8081 \
  -host 127.0.0.1 -port 8090 -threads 2 -ctx-size 2048
```

Supervisor status:

```bash
curl --fail --silent --show-error http://127.0.0.1:8090/healthz
curl --fail --silent --show-error http://127.0.0.1:8090/status
```

The native llama.cpp API remains available at `http://127.0.0.1:8081`.

## 4. Verify native inference through the supervised process

```bash
curl --fail --silent --show-error http://127.0.0.1:8081/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{"model":"opencompute-local","messages":[{"role":"user","content":"Explain OpenCompute in one sentence."}],"temperature":0,"max_tokens":64,"stream":false}'
```

Run the same request again. The model should remain loaded between requests.

## 5. Record service behaviour

Capture:
- server startup/model-load output;
- supervisor health/status response;
- two inference responses and timings;
- process RSS while idle and during inference;
- graceful shutdown using Ctrl+C;
- confirmation that both the supervisor and child process exit.

## Security and scope

- Do not bind either service to `0.0.0.0`, a LAN interface, or a public address.
- Do not expose the llama.cpp server directly to the internet.
- This is a local trusted-lab experiment with no authentication.
- Do not put activation payloads or model execution through the Go coordinator.
- This milestone does not yet integrate the existing synthetic worker protocol.
- A successful HTTP inference test proves a persistent native inference service, not model-parallel execution.

## M1 acceptance for this slice

- [x] `llama-server` builds from the pinned runtime.
- [x] Server starts and loads the selected model once.
- [x] Native `/health` reports healthy.
- [x] Chat-completion request succeeds.
- [ ] Go supervisor builds and starts the native child process.
- [ ] Supervisor health reports ready only while native runtime is healthy.
- [ ] Two sequential inference requests succeed through the supervised process.
- [ ] Supervisor shuts down the child cleanly.
- [ ] Startup, request, memory and timing evidence is recorded.
