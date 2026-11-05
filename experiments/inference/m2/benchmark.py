#!/usr/bin/env python3
"""Concurrent request-level OpenCompute inference benchmark (stdlib only)."""
import argparse
import concurrent.futures
import json
import statistics
import time
import urllib.error
import urllib.request


def percentile(values, fraction):
    if not values:
        return None
    ordered = sorted(values)
    index = max(0, min(len(ordered) - 1, int(round((len(ordered) - 1) * fraction))))
    return ordered[index]


def run_one(args, payload):
    body = json.dumps(payload).encode("utf-8")
    headers = {"Content-Type": "application/json"}
    if args.token:
        headers["Authorization"] = "Bearer " + args.token
    request = urllib.request.Request(args.url, data=body, headers=headers, method="POST")
    started = time.perf_counter()
    try:
        with urllib.request.urlopen(request, timeout=args.timeout) as response:
            raw = response.read()
            status = response.status
        elapsed = time.perf_counter() - started
        parsed = json.loads(raw.decode("utf-8"))
        tokens = int((parsed.get("usage") or {}).get("completion_tokens") or 0)
        return {"ok": 200 <= status < 400, "status": status, "latency_s": elapsed, "completion_tokens": tokens}
    except urllib.error.HTTPError as error:
        elapsed = time.perf_counter() - started
        return {"ok": False, "status": error.code, "latency_s": elapsed, "completion_tokens": 0, "error": error.read(512).decode("utf-8", "replace")}
    except Exception as error:  # network and response decoding failures are benchmark results
        elapsed = time.perf_counter() - started
        return {"ok": False, "status": 0, "latency_s": elapsed, "completion_tokens": 0, "error": str(error)}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--url", required=True, help="Coordinator /api/v1/inference/chat/completions URL")
    parser.add_argument("--token", default="", help="Coordinator bearer token")
    parser.add_argument("--model", default="opencompute")
    parser.add_argument("--prompt", default="Explain OpenCompute in two concise sentences.")
    parser.add_argument("--requests", type=int, default=20)
    parser.add_argument("--concurrency", type=int, default=4)
    parser.add_argument("--max-tokens", type=int, default=96)
    parser.add_argument("--timeout", type=float, default=180)
    args = parser.parse_args()
    if args.requests < 1 or args.concurrency < 1:
        parser.error("--requests and --concurrency must be positive")

    payload = {
        "model": args.model,
        "messages": [{"role": "user", "content": args.prompt}],
        "temperature": 0,
        "max_tokens": args.max_tokens,
        "stream": False,
    }
    started = time.perf_counter()
    with concurrent.futures.ThreadPoolExecutor(max_workers=args.concurrency) as pool:
        results = list(pool.map(lambda _: run_one(args, payload), range(args.requests)))
    elapsed = time.perf_counter() - started
    latencies = [item["latency_s"] for item in results]
    successes = sum(1 for item in results if item["ok"])
    tokens = sum(item["completion_tokens"] for item in results)
    output = {
        "requests": args.requests,
        "concurrency": args.concurrency,
        "successes": successes,
        "failures": args.requests - successes,
        "wall_time_s": round(elapsed, 4),
        "requests_per_second": round(successes / elapsed, 4) if elapsed else 0,
        "completion_tokens": tokens,
        "completion_tokens_per_second": round(tokens / elapsed, 4) if elapsed else 0,
        "latency_ms": {
            "mean": round(statistics.mean(latencies) * 1000, 2) if latencies else None,
            "median": round(statistics.median(latencies) * 1000, 2) if latencies else None,
            "p95": round(percentile(latencies, 0.95) * 1000, 2) if latencies else None,
        },
        "status_counts": {str(code): sum(1 for item in results if item["status"] == code) for code in sorted({item["status"] for item in results})},
        "errors": [item.get("error") for item in results if item.get("error")][:5],
    }
    print(json.dumps(output, indent=2))
    return 0 if successes == args.requests else 1


if __name__ == "__main__":
    raise SystemExit(main())
