# M2 — Request-level inference routing

M2 distributes independent chat-completion requests across already-running inference workers. It does not split one model across workers; that is the later M3 model-parallel milestone and remains blocked on the M0 GGML RPC feasibility gate.

## Request path

`dashboard -> authenticated Go coordinator -> selected worker gateway -> loopback llama-server`

- The coordinator filters workers by `kind=inference`, a heartbeat no older than 30 seconds, and the advertised `chat-completions` operation.
- `POST /api/v1/inference/chat/completions` selects eligible workers round-robin.
- `POST /api/v1/inference/workers/{id}/chat/completions` targets one worker explicitly.
- The coordinator forwards the OpenAI-compatible JSON request and streams the response.
- The worker gateway authenticates the coordinator bearer token, then forwards to its local llama-server. The coordinator token is not forwarded to llama-server.
- The native llama-server remains bound to loopback. Only the small worker gateway is reachable over the lab network.
- The coordinator never handles model activations or KV-cache data.

## Configure a worker for a remote coordinator

The worker gateway must listen on an address reachable from the coordinator. Use a trusted private LAN and a strong shared token. Keep the native llama-server on loopback.

Example for a Linux host where the coordinator runs on a different machine:

```bash
export OCF_API_TOKEN='use-a-long-random-lab-secret'
go run ./cmd/inference-worker \
  -coordinator http://COORDINATOR_LAN_IP:8080 \
  -name linux-inference-01 \
  -host 0.0.0.0 -port 8090 \
  -advertise-endpoint http://LINUX_HOST_LAN_IP:8090 \
  -native-host 127.0.0.1 -native-port 8081 \
  -threads 2 -ctx-size 2048
```

The coordinator must use the same token. Do not expose worker gateway or coordinator ports to the public internet. Restrict inbound traffic to the trusted lab network with the host firewall. Do not publish the native llama-server port.

For multiple workers, use unique native and gateway ports and unique worker names. Each worker process must have a separate model/runtime resource budget. Containers on one host share physical RAM; they do not create additional memory capacity.

## API examples

Round-robin request:

```bash
curl --fail-with-body http://COORDINATOR_LAN_IP:8080/api/v1/inference/chat/completions \
  -H "Authorization: Bearer $OCF_API_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"model":"opencompute","messages":[{"role":"user","content":"Explain OpenCompute."}],"temperature":0,"max_tokens":128,"stream":false}'
```

Pin a request to a worker by using its coordinator worker ID:

```text
POST /api/v1/inference/workers/WORKER_ID/chat/completions
```

## Sessions and recovery

Create a coordinator session with `POST /api/v1/inference/sessions`, then send turns to `POST /api/v1/inference/sessions/{id}/chat/completions`. Sessions are pinned to one healthy worker. If a worker is lost, the next request is assigned to another worker and the session epoch increments. The client must resend the full conversation history. See [SESSION-SEMANTICS.md](SESSION-SEMANTICS.md).

This is routing affinity and client-assisted replay metadata, **not KV-cache migration**. Session records are in-memory and are lost on coordinator restart. A failed in-flight generation is returned as an error and is not retried automatically.

## Metrics and benchmark

Authenticated `GET /api/v1/metrics/inference` returns per-worker request totals, successes, failures, in-flight requests, average latency, last status and last request time.

Run a concurrent request-level benchmark from the repository root:

```bash
python3 experiments/inference/m2/benchmark.py \
  --url http://COORDINATOR_LAN_IP:8080/api/v1/inference/chat/completions \
  --token "$OCF_API_TOKEN" \
  --requests 40 --concurrency 4 --max-tokens 96
```

The script reports request success/failure counts, mean/median/p95 latency, aggregate requests/sec and completion tokens/sec (when the upstream reports token usage). Run several concurrency levels and compare identical model, prompt and decoding settings. This is request-level throughput; it is not a model-parallel benchmark.

## Operational semantics and limitations

- Round-robin is request-level distribution, not load-aware placement. It does not measure queue depth, CPU pressure, model memory or network throughput.
- A request is not retried automatically after a worker failure. Retrying a generation request can duplicate work and should be an explicit client decision.
- Chat completions are independent requests. There is no persistent conversation/session ID or KV-cache affinity across multiple requests yet.
- Worker heartbeat is a liveness signal, not a proof that the model can complete a generation.
- Worker registration currently advertises a model filename. The upstream llama.cpp model alias can differ; the gateway passes the request's model field through unchanged.
- The shared bearer token is a lab trust mechanism, not per-worker identity or production authentication.
- The worker gateway health endpoints are diagnostic and should remain on the trusted lab network.
- This milestone does not provide model parallelism, tensor splitting, layer placement, activation transport, or KV-cache migration.

## M2 acceptance

- [x] Inference workers can be distinguished from compute workers.
- [x] Dashboard displays model/runtime/capability metadata.
- [x] Coordinator selects healthy inference workers round-robin for independent requests.
- [x] A caller can explicitly target a worker.
- [x] Coordinator request authentication is enforced when configured.
- [x] Worker gateway authenticates forwarded requests and strips the shared token before calling llama-server.
- [x] Native runtime remains loopback-bound.
- [x] Sessions pin to one worker and increment an epoch when reassigned.
- [x] Per-worker request telemetry is exposed to the dashboard.
- [x] A concurrent request-level benchmark harness is included.
- [ ] Validate end-to-end from the MacBook dashboard to a Linux worker on the user's private LAN.
- [ ] Run multiple workers and confirm round-robin distribution with per-worker request counters/logs.
- [ ] Benchmark aggregate throughput with independent requests.

M3 is not complete until a single inference request genuinely executes across multiple model partitions and passes the correctness/capacity experiment.
