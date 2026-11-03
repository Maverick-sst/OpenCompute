# OpenCompute Fabric

An open protocol for coordinating spare compute resources across independently operated machines.

**Current stage:** V0 coordinator and trusted local worker protocol. This is an experimental foundation, not a public compute marketplace.

## V0 capabilities

- Submit jobs with a bounded number of synthetic records.
- Deterministically partition jobs into non-overlapping batches.
- Register workers and report heartbeats.
- Pull work through expiring leases.
- Validate worker ownership and fencing tokens when accepting results.
- Reassign expired work and reject stale completions.
- Run a deterministic CPU workload using the included local worker.

## Architecture

- **Coordinator:** Go HTTP API; owns job lifecycle, workers, and leases.
- **Worker agent:** Go process that registers, polls for leases, executes a deterministic synthetic workload, and reports results.
- **Batch engine:** deterministic range partitioning.
- **Storage:** atomic local JSON state file for single-process V0; set `OCF_STATE_PATH` to choose its location. Use a persistent disk; do not share the file between coordinator replicas.
- **Execution:** synthetic code only. OCI execution is a later milestone.
- **Dashboard:** React/Vite control panel in `dashboard/`; GitHub Pages deployment workflow included.

## Run locally

Requires Go 1.23 or newer.

Terminal 1:

```bash
go test ./...
OCF_STATE_PATH=./data/ocf-state.json go run ./cmd/coordinator
```

Terminal 2:

```bash
go run ./cmd/worker -coordinator http://127.0.0.1:8080 -name laptop-worker
```

Submit one million records:

```bash
curl -X POST http://127.0.0.1:8080/api/v1/jobs \
  -H 'Content-Type: application/json' \
  -d '{"total_items":1000000,"batch_size":100000}'
```

Inspect job state using the returned ID:

```bash
curl http://127.0.0.1:8080/api/v1/jobs/JOB_ID
```

## API

- `GET /healthz` — coordinator health.
- `POST /api/v1/jobs` — submit a job.
- `GET /api/v1/jobs/{id}` — inspect job and batch states.
- `POST /api/v1/workers` — register a worker.
- `POST /api/v1/workers/{id}/heartbeat` — heartbeat.
- `POST /api/v1/workers/{id}/leases` — claim one batch, or receive 204 when idle.
- `POST /api/v1/leases/{id}/complete` — report batch completion.

See [docs/WORKER-PROTOCOL.md](docs/WORKER-PROTOCOL.md) and [docs/V0-ROADMAP.md](docs/V0-ROADMAP.md). The dashboard runs with `cd dashboard && npm install && npm run dev`.

## Security limitations

Set `OCF_API_TOKEN` to require a shared bearer token on all `/api/v1/*` routes; pass the same value to the worker through `OCF_API_TOKEN` or `-token`. The dashboard asks for this token in the browser and keeps it in memory only. Set `OCF_ALLOWED_ORIGINS` to a comma-separated list of allowed dashboard origins. For GitHub Pages, set the repository variable `OCF_API_BASE_URL` to the deployed coordinator URL and allow `https://maverick-sst.github.io` in the coordinator's CORS configuration. The coordinator refuses to bind beyond localhost without this token. This shared-token mechanism is a basic V0 access gate, not a production identity or multi-tenant trust system. TLS, signed worker credentials, workload sandboxing, resource enforcement, durable state, and provider trust are not implemented. Containers share the host kernel and are not a confidentiality boundary. Never send sensitive data to untrusted workers.

## GitHub Pages dashboard

The `Deploy dashboard to GitHub Pages` workflow publishes the static dashboard when dashboard files are merged to `main`. In repository Settings → Pages, choose **GitHub Actions** as the deployment source. GitHub Pages hosts only the static frontend; the coordinator API must run separately on a host with persistent storage.

## License

TBD. No license has been selected yet.
