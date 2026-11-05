# M2.4 — Inference session affinity and worker-loss handling

This slice adds coordinator-owned session assignment metadata for request-level inference.

## API

Create a session:

```bash
curl --fail-with-body -X POST "$OCF_COORDINATOR/api/v1/inference/sessions" \
  -H "Authorization: Bearer $OCF_API_TOKEN"
```

The response includes an opaque session ID, assigned worker ID, epoch and reassignment count.

Send subsequent turns to the session:

```text
POST /api/v1/inference/sessions/SESSION_ID/chat/completions
```

Read session placement:

```text
GET /api/v1/inference/sessions/SESSION_ID
```

The coordinator keeps the session pinned to its assigned healthy worker. If the worker is stale or a proxy connection fails, sessions owned by that worker are invalidated. On the next session request, the coordinator assigns a healthy worker and increments the session epoch. Responses include `X-OpenCompute-Session-ID` and `X-OpenCompute-Session-Epoch`.

## Recovery contract

- A failed in-flight generation returns an error; the coordinator does not silently retry it.
- The next request can be reassigned to a healthy worker.
- The client must send the full conversation message history on each request so the new worker can reconstruct context by replaying the prompt.
- A higher epoch fences off the old coordinator-side session assignment. It does not transfer native runtime state.

## Important boundary: this is not KV-cache migration

The current public llama.cpp server integration does not expose an OpenCompute session-to-KV-cache ownership API. This milestone therefore implements **routing affinity and replay metadata only**. It does not preserve or migrate the old worker's KV cache, guarantee prompt-cache reuse, or restore generation state after a crash. The session registry is currently in-memory and is lost when the coordinator restarts.

Do not describe this as transparent continuation of a generation. It is an explicit, client-assisted replay path. True KV-cache ownership/transfer remains blocked on the M0/M3 native-runtime feasibility work.

## Validation

Automated tests cover session creation, worker pinning, worker invalidation, epoch increment, reassignment and missing-session handling. End-to-end worker termination and replay still need the Linux lab.
