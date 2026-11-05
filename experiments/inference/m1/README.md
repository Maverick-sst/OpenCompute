# M1 — Persistent native inference worker

M1 turns the local llama.cpp runtime into a supervised OpenCompute inference worker and registers its capabilities with the existing coordinator. The Go coordinator remains a control plane; prompts and model execution stay on the worker's native HTTP endpoint.

## Lab assumptions

- llama.cpp checkout: `~/src/llama.cpp`
- pinned runtime commit: `2bc46baa9207d098d03741ac394e7faaf1a955b6`
- build directory: `build-opencompute`
- smoke-test model: `~/models/opencompute/qwen2.5-0.5b-instruct-q4_k_m.gguf`
- native inference bind: `127.0.0.1:8081`
- worker health bind: `127.0.0.1:8090`
- CPU threads: 2

## Build and test

From the OpenCompute repository root:

```bash
bash experiments/inference/m1/build-llama-server.sh
gofmt -w $(find internal/inference cmd/inference-worker internal/jobs internal/api -name '*.go' -type f)
go test ./...
go vet ./...
```

## Start the coordinator

Use a local token for the lab. Keep the coordinator on loopback for this experiment:

```bash
export OCF_API_TOKEN='replace-with-a-local-development-token'
export OCF_ADDR='127.0.0.1:8080'
go run ./cmd/coordinator
```

In another terminal, use the same token and start the inference worker:

```bash
export OCF_API_TOKEN='replace-with-a-local-development-token'
go run ./cmd/inference-worker \
  -coordinator http://127.0.0.1:8080 \
  -llama-server "$HOME/src/llama.cpp/build-opencompute/bin/llama-server" \
  -model "$HOME/models/opencompute/qwen2.5-0.5b-instruct-q4_k_m.gguf" \
  -native-host 127.0.0.1 -native-port 8081 \
  -host 127.0.0.1 -port 8090 -threads 2 -ctx-size 2048 \
  -heartbeat 5s -memory-mb 1024
```

The worker loads the model first, registers itself as `kind=inference`, and sends coordinator heartbeats every five seconds. Registration includes runtime, model filename, inference endpoint and supported operations. The default registration is local-lab only: the advertised endpoint is loopback and is reachable only from the same host.

## Verify registration and health

```bash
curl --fail --silent --show-error http://127.0.0.1:8080/healthz
curl --fail --silent --show-error http://127.0.0.1:8080/api/v1/workers \
  -H "Authorization: Bearer $OCF_API_TOKEN"
curl --fail --silent --show-error http://127.0.0.1:8090/healthz
curl --fail --silent --show-error http://127.0.0.1:8090/status
```

Expected worker metadata includes:

- `kind: inference`
- `runtime: llama.cpp`
- model filename
- `inference_endpoint: http://127.0.0.1:8081`
- `operations: ["chat-completions", "completion"]`
- `last_seen_at`, refreshed by heartbeat

Existing clients that omit `kind` continue to register as `compute`. The synthetic lease protocol is unchanged. The registry currently records last-seen timestamps but does not expire or deregister stale workers.

## Verify inference

```bash
curl --fail --silent --show-error http://127.0.0.1:8081/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{"model":"opencompute-local","messages":[{"role":"user","content":"Explain OpenCompute in one sentence."}],"temperature":0,"max_tokens":64,"stream":false}'
```

Run it twice to verify the model remains loaded. The coordinator does not proxy this request.

## Shutdown

Press Ctrl+C in the worker terminal. Verify that the worker and native child exit:

```bash
pgrep -af 'inference-worker|llama-server'
```

Stop the coordinator separately.

## Security and scope

- Keep coordinator, worker health API and native inference API bound to loopback for this local experiment.
- The worker health API is not authenticated; it is loopback-only.
- Never expose the native llama.cpp server directly to the internet.
- A loopback inference endpoint advertised to the coordinator is valid only when both processes run on the same host.
- Do not route prompts, activations, tensors or model execution through the coordinator.
- Coordinator registration is discovery metadata, not a distributed inference scheduler.
- No worker expiry, deregistration, request routing, model parallelism or multi-host networking is implemented in M1.

## M1 acceptance checklist

- [x] Pinned llama.cpp runtime builds.
- [x] Native model loads and serves chat-completion requests.
- [x] Go supervisor launches and shuts down the native child.
- [x] Worker health reports runtime readiness.
- [x] Coordinator supports compute and inference worker records.
- [x] Inference capabilities persist in the coordinator file store.
- [x] Inference worker registers after native runtime becomes ready.
- [x] Worker sends periodic heartbeats.
- [ ] Local end-to-end coordinator + worker verification on the lab machine.
- [ ] `go test ./...` and `go vet ./...` after pulling this branch.

M1 proves a supervised, registered, single-node inference worker. It does not prove multi-node scheduling or model-parallel inference.
