# OpenCompute Fabric

An open protocol for coordinating spare compute resources across independently operated machines.

> **Current stage:** V0 foundation. The first milestone validates job orchestration and deterministic batch execution before introducing untrusted external workers, model inference, or payments.

## V0 goals

- Submit a bounded batch job to a coordinator.
- Partition work into deterministic, non-overlapping ranges.
- Track job state and expose a small HTTP API.
- Establish a testable Go codebase and CI pipeline.
- Keep execution adapters separate from scheduling and protocol logic.

## Planned architecture

- **Coordinator:** Go HTTP API; owns job lifecycle and scheduling decisions.
- **Worker agent:** independently running process that advertises capacity, polls for leases, executes allowed workloads, and reports results.
- **Batch engine:** deterministic partitioning and result aggregation.
- **Dashboard:** static web UI consuming the coordinator API.
- **Execution:** OCI-compatible containers in a later milestone. The initial local executor is deliberately limited and is not a security boundary.

## Development

The V0 coordinator requires Go 1.23 or newer.

```bash
go test ./...
go run ./cmd/coordinator
```

The API listens on `127.0.0.1:8080` by default.

- `GET /healthz` — health check
- `POST /api/v1/jobs` — submit a batch job
- `GET /api/v1/jobs/{id}` — retrieve job status

See [docs/](docs/) for protocol notes and the V0 roadmap.

## Security boundary

Do not submit confidential data to machines you do not control. Containers share the host kernel and are not, by themselves, a confidentiality guarantee. V0 uses an in-memory coordinator store and simulated/local execution only; it is not production-ready for public workloads or payments.

## License

TBD. No license has been selected yet.
