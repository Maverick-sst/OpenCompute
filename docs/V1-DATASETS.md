# V1 dataset ingestion foundation

This branch introduces the first V1 data-plane primitive while preserving the existing V0 synthetic job API.

## Upload

`POST /api/v1/datasets` accepts multipart form data with a `file` field containing JSONL. The coordinator streams the upload to a temporary file, enforces a 256 MiB development limit, computes SHA-256, validates each nonblank line as a JSON object, and writes the source, metadata and deterministic manifest to `OCF_DATASET_ROOT` (default `./data/datasets`).

Blank lines are ignored for record counts but remain covered by byte offsets. CRLF is accepted. A final record does not need a trailing newline. A single record may not exceed 1 MiB. Partitions are formed on newline boundaries around a 4 MiB target; a long record can make a partition larger than the target.

## Endpoints

- `POST /api/v1/datasets` — upload JSONL; returns dataset metadata and its partitions.
- `GET /api/v1/datasets/{id}` — retrieve metadata.
- `GET /api/v1/datasets/{id}/partitions` — retrieve the manifest.
- `GET /api/v1/partitions/{id}/data` — stream the bytes for a manifest partition.

All dataset endpoints use the existing bearer-token middleware. The partition streaming endpoint is currently a coordinator-authenticated development primitive; it is not yet lease-owner-gated. Do not expose it to untrusted workers or networks. V1 job creation, lease assignment, worker-side analytics, lease renewal, aggregation and dashboard dataset workflow remain subsequent integration work.

## Local configuration

```bash
OCF_DATASET_ROOT=./data/datasets
OCF_STATE_PATH=./data/ocf-state.json
OCF_API_TOKEN=...
```

Dataset bytes are stored outside the JSON coordinator state file.
