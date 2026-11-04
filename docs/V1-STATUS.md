# V1 implementation status

## Dataset foundation

The coordinator accepts JSONL uploads at `POST /api/v1/datasets`. Uploaded source bytes, metadata, and partition manifests are stored under `OCF_DATASET_ROOT`, separately from the V0 coordinator state file. Dataset metadata includes byte size, record count, SHA-256, and readiness status.

Partition boundaries are newline-aligned and byte offsets are half-open. Blank lines are retained in the source and byte coverage but excluded from record counts. CRLF and a final line without a trailing newline are supported.

## Log analytics library

`internal/workload/logs` contains a streaming parser and associative partial-aggregate merge for log records. It tracks records read/processed/rejected, ERROR-level records, status/service/endpoint counts, and latency sum/count/max. Average latency is calculated from merged sum/count, not by averaging worker averages.

## Dashboard preview

The dashboard now has a JSONL upload panel and displays upload metadata and partition count for uploads made during the current browser session. This panel is an ingestion preview only; it does not yet submit dataset-backed jobs or show distributed analytics results.

## Explicitly not complete

- Dataset-backed scheduler tasks and leases.
- Lease-owner checks on partition downloads.
- Worker partition fetching and execution.
- Lease renewal, retries and stale-result fencing for V1 results.
- Coordinator-side partial-result persistence and final aggregation endpoint.
- End-to-end dashboard job creation/results.
- Full local Docker Compose orchestration and real multi-worker integration test.

The partition stream endpoint is authenticated by the shared coordinator token but is not yet bound to a specific active worker lease. Keep it on a trusted local network only.
