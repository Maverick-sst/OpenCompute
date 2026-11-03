# V0 worker protocol

## Worker lifecycle

1. Register using `POST /api/v1/workers`.
2. Heartbeat and poll for work using `POST /api/v1/workers/{id}/leases`.
3. Receive one batch lease or `204 No Content` if no work is available.
4. Execute the deterministic synthetic CPU workload.
5. Complete the lease using `POST /api/v1/leases/{lease_id}/complete`.

Leases last 30 seconds. Expired leases are returned to the queue and reassigned with a monotonically increasing fencing token. A stale lease cannot be completed after reassignment.

## Run locally

Terminal 1:

```bash
go run ./cmd/coordinator
```

Terminal 2:

```bash
go run ./cmd/worker -coordinator http://127.0.0.1:8080 -name laptop-worker
```

Submit one million synthetic records:

```bash
curl -X POST http://127.0.0.1:8080/api/v1/jobs \
  -H 'Content-Type: application/json' \
  -d '{"total_items":1000000,"batch_size":100000}'
```

Then inspect the returned job ID:

```bash
curl http://127.0.0.1:8080/api/v1/jobs/JOB_ID
```

## API

- `POST /api/v1/workers` — register a worker with name, CPU cores, and memory MB.
- `POST /api/v1/workers/{id}/heartbeat` — update last-seen time.
- `POST /api/v1/workers/{id}/leases` — claim one queued batch.
- `POST /api/v1/leases/{id}/complete` — report worker ID, fencing token, processed count, and checksum.

## Security limitations

This protocol is for trusted local development only. Worker IDs are identifiers, not credentials; endpoints are unauthenticated and must not be exposed to the public internet. TLS, signed worker credentials, workload isolation, resource enforcement, durable state, and provider trust are not implemented. Never use this iteration with sensitive data or untrusted public workers.
