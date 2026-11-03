# V0 roadmap

## Milestone 0 — protocol foundation

- [x] Repository baseline and CI
- [x] Coordinator health endpoint
- [x] Job submission and lookup
- [x] Deterministic batch partitioning
- [ ] Persistent job store
- [ ] Worker registration and heartbeat
- [ ] Lease creation, expiry, and fencing tokens
- [ ] Worker polling and result reporting
- [ ] Retry and failed-worker recovery
- [ ] Dashboard and live job view

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
