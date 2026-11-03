# V0 roadmap

## Milestone 0 — protocol foundation

- [x] Repository baseline and CI
- [x] Coordinator health endpoint
- [x] Job submission and lookup
- [x] Deterministic batch partitioning
- [x] Atomic local JSON persistence for one coordinator process
- [x] Worker registration and heartbeat
- [x] Lease creation, expiry, and fencing tokens
- [x] Worker polling and result reporting
- [x] Expired-lease recovery and stale completion rejection
- [x] Dashboard and live job view (initial control panel)
- [ ] Multi-replica durable database and scheduler
- [ ] Worker credentials, TLS deployment, and stronger identity

## First experiment: one million records

Use a deterministic CPU-bound transformation before introducing model inference. Submit 1,000,000 records and compare batch sizes (for example 10k, 50k, 100k, and 250k). Record scheduling overhead, throughput, completion time, retries, and worker utilization. Do not send the entire dataset through an LLM; add a small-model inference adapter only after the scheduler and data path are measured.

## Trust and execution

The first coordinator stores jobs in memory and only creates batch plans. It does not execute arbitrary code. Later worker execution must use explicit workload allowlists, resource limits, lease fencing, and isolated runtime adapters. Containers share the host kernel and are not a confidentiality boundary. Never place sensitive user data on untrusted provider machines.

## Non-goals for initial V0

- Token issuance or real payouts
- Global shared RAM abstraction
- Distributed model pretraining
- Kubernetes dependency
- Claiming container isolation as a security guarantee
