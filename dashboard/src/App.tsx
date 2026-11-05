import { useCallback, useEffect, useMemo, useState, type FormEvent } from "react";

type Job = {
  id: string;
  total_items: number;
  batch_size: number;
  status: string;
  batches: { status: string; start: number; end: number }[];
  created_at: string;
};

type Dataset = { id: string; filename: string; size_bytes: number; record_count: number; checksum: string; status: string; partitions: number };

type Worker = {
  id: string;
  name: string;
  kind?: "compute" | "inference";
  cpu_cores: number;
  memory_mb: number;
  runtime?: string;
  model?: string;
  inference_endpoint?: string;
  operations?: string[];
  registered_at?: string;
  last_seen_at: string;
};

const API_DEFAULT = import.meta.env.VITE_API_BASE_URL || "http://127.0.0.1:8080";

export default function App() {
  const [apiBase, setApiBase] = useState(API_DEFAULT);
  const [token, setToken] = useState("");
  const [jobs, setJobs] = useState<Job[]>([]);
  const [workers, setWorkers] = useState<Worker[]>([]);
  const [datasets, setDatasets] = useState<Dataset[]>([]);
  const [uploading, setUploading] = useState(false);
  const [totalItems, setTotalItems] = useState("1000000");
  const [batchSize, setBatchSize] = useState("100000");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null);
  const [selectedInferenceWorker, setSelectedInferenceWorker] = useState("");
  const [inferencePrompt, setInferencePrompt] = useState("Explain how OpenCompute distributes inference requests.");
  const [inferenceOutput, setInferenceOutput] = useState("");
  const [inferenceBusy, setInferenceBusy] = useState(false);

  const headers = useMemo(() => {
    const value: Record<string, string> = { "Content-Type": "application/json" };
    if (token.trim()) value.Authorization = `Bearer ${token.trim()}`;
    return value;
  }, [token]);

  const refresh = useCallback(async () => {
    setBusy(true);
    setError("");
    try {
      const base = apiBase.trim().replace(/\/$/, "");
      const [jobResponse, workerResponse] = await Promise.all([
        fetch(`${base}/api/v1/jobs`, { headers }),
        fetch(`${base}/api/v1/workers`, { headers }),
      ]);
      if (!jobResponse.ok || !workerResponse.ok) {
        throw new Error(`Coordinator returned ${jobResponse.status}/${workerResponse.status}`);
      }
      const [jobData, workerData] = await Promise.all([
        jobResponse.json() as Promise<Job[]>,
        workerResponse.json() as Promise<Worker[]>,
      ]);
      setJobs(jobData);
      setWorkers(workerData);
      setLastUpdated(new Date());
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to reach coordinator");
    } finally {
      setBusy(false);
    }
  }, [apiBase, headers]);

  useEffect(() => {
    void refresh();
    const id = window.setInterval(() => void refresh(), 5000);
    return () => window.clearInterval(id);
  }, [refresh]);

  async function uploadDataset(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = event.currentTarget;
    const selected = form.elements.namedItem("dataset") as HTMLInputElement;
    const file = selected.files?.[0];
    if (!file) return;
    setUploading(true); setError("");
    try {
      const uploadHeaders: Record<string, string> = {};
      if (token.trim()) uploadHeaders.Authorization = `Bearer ${token.trim()}`;
      const response = await fetch(`${apiBase.trim().replace(/\/$/, "")}/api/v1/datasets`, { method: "POST", headers: uploadHeaders, body: (() => { const data = new FormData(); data.append("file", file); return data; })() });
      if (!response.ok) throw new Error(`Dataset upload failed (${response.status}): ${await response.text()}`);
      const result = await response.json() as { dataset: Omit<Dataset, "partitions">; partitions: unknown[] };
      setDatasets((current) => [{ ...result.dataset, partitions: result.partitions.length }, ...current]);
      form.reset();
    } catch (err) { setError(err instanceof Error ? err.message : "Unable to upload dataset"); }
    finally { setUploading(false); }
  }

  async function submitJob(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    try {
      const response = await fetch(`${apiBase.trim().replace(/\/$/, "")}/api/v1/jobs`, {
        method: "POST",
        headers,
        body: JSON.stringify({
          total_items: Number(totalItems),
          batch_size: Number(batchSize),
        }),
      });
      if (!response.ok) throw new Error(`Job submission failed (${response.status})`);
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to submit job");
    }
  }

  const activeWorkers = workers.filter(
    (worker) => Date.now() - new Date(worker.last_seen_at).getTime() < 30_000,
  ).length;
  const availableInferenceWorkers = workers.filter((worker) =>
    worker.kind === "inference" &&
    Date.now() - new Date(worker.last_seen_at).getTime() < 30_000 &&
    worker.operations?.includes("chat-completions"),
  );
  const inferenceWorkers = workers.filter((worker) => worker.kind === "inference").length;
  const computeWorkers = workers.length - inferenceWorkers;
  const completedBatches = jobs.reduce(
    (sum, job) => sum + job.batches.filter((batch) => batch.status === "completed").length,
    0,
  );
  const totalBatches = jobs.reduce((sum, job) => sum + job.batches.length, 0);

  async function runInference(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const worker = availableInferenceWorkers.find((item) => item.id === selectedInferenceWorker) ?? availableInferenceWorkers[0];
    if (!worker) {
      setError("No healthy inference worker advertises chat-completions");
      return;
    }
    setInferenceBusy(true);
    setInferenceOutput("");
    setError("");
    try {
      const response = await fetch(`${apiBase.trim().replace(/\\/$/, "")}/api/v1/inference/workers/${worker.id}/chat/completions`, {
        method: "POST",
        headers,
        body: JSON.stringify({
          model: worker.model,
          messages: [{ role: "user", content: inferencePrompt }],
          temperature: 0,
          max_tokens: 128,
          stream: false,
        }),
      });
      if (!response.ok) throw new Error(`Inference request failed (${response.status}): ${await response.text()}`);
      const result = await response.json() as { choices?: { message?: { content?: string } }[] };
      setInferenceOutput(result.choices?.[0]?.message?.content ?? JSON.stringify(result, null, 2));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Inference request failed");
    } finally {
      setInferenceBusy(false);
    }
  }


  return (
    <main className="shell">
      <header className="topbar">
        <div>
          <div className="eyebrow">OPEN COMPUTE FABRIC / V1 PREVIEW</div>
          <h1>Compute, coordinated.</h1>
          <p className="subtitle">A live view of jobs, batches, and participating workers.</p>
        </div>
        <div className="connection">
          <span className={error ? "dot offline" : "dot"} />
          {error ? "Disconnected" : "Coordinator"}
        </div>
      </header>

      <section className="config panel">
        <label>
          Coordinator URL
          <input value={apiBase} onChange={(e) => setApiBase(e.target.value)} placeholder="https://your-api.example.com" />
        </label>
        <label>
          API token <span className="muted">(kept in this tab only)</span>
          <input type="password" value={token} onChange={(e) => setToken(e.target.value)} placeholder="Bearer token" autoComplete="off" />
        </label>
        <button className="secondary" onClick={() => void refresh()} disabled={busy}>
          {busy ? "Refreshing…" : "Refresh"}
        </button>
      </section>

      {error && <div className="error">{error}. Check the API URL, token, and OCF_ALLOWED_ORIGINS.</div>}

      <section className="stats">
        <article className="stat panel"><span>Registered workers</span><strong>{workers.length}</strong><small>{activeWorkers} heartbeats in the last 30 seconds</small></article>
        <article className="stat panel"><span>Inference nodes</span><strong>{inferenceWorkers}</strong><small>{computeWorkers} compute-only nodes</small></article>
        <article className="stat panel"><span>Jobs submitted</span><strong>{jobs.length}</strong><small>Across this coordinator</small></article>
        <article className="stat panel"><span>Batches completed</span><strong>{completedBatches}<em> / {totalBatches}</em></strong><small>Completed batch leases</small></article>
      </section>

      <section className="panel section inference-console">
        <div className="section-heading">
          <div><div className="eyebrow">REQUEST-LEVEL INFERENCE · M2</div><h2>Run an inference request</h2></div>
          <span className="count">{availableInferenceWorkers.length} ready</span>
        </div>
        <form onSubmit={runInference} className="inference-form">
          <label>Inference worker
            <select value={selectedInferenceWorker} onChange={(event) => setSelectedInferenceWorker(event.target.value)}>
              <option value="">Auto-select first healthy worker</option>
              {availableInferenceWorkers.map((worker) => <option key={worker.id} value={worker.id}>{worker.name} · {worker.model}</option>)}
            </select>
          </label>
          <label>Prompt
            <textarea value={inferencePrompt} onChange={(event) => setInferencePrompt(event.target.value)} rows={3} required />
          </label>
          <button type="submit" disabled={inferenceBusy || availableInferenceWorkers.length === 0 || !inferencePrompt.trim()}>
            {inferenceBusy ? "Generating…" : "Send inference request ↗"}
          </button>
        </form>
        {availableInferenceWorkers.length === 0 && <p className="hint">Start a healthy inference worker that advertises chat-completions to enable this console.</p>}
        {inferenceOutput && <div className="inference-output"><div className="eyebrow">MODEL RESPONSE</div><p>{inferenceOutput}</p></div>}
        <p className="hint">The coordinator authenticates and forwards the request to the selected worker. Native inference stays on the worker; activation and KV-cache traffic are not routed through the coordinator.</p>
      </section>

      <section className="content-grid">
        <article className="panel section">
          <div className="section-heading">
            <div><div className="eyebrow">SCHEDULER</div><h2>Submit a workload</h2></div>
          </div>
          <form onSubmit={submitJob} className="job-form">
            <label>Total records<input type="number" min="1" value={totalItems} onChange={(e) => setTotalItems(e.target.value)} required /></label>
            <label>Batch size<input type="number" min="1" value={batchSize} onChange={(e) => setBatchSize(e.target.value)} required /></label>
            <button type="submit">Create batch job <span>↗</span></button>
          </form>
          <p className="hint">V0 synthetic range jobs remain available while V1 dataset processing is integrated.</p>
        </article>

        <article className="panel section">
          <div className="section-heading"><div><div className="eyebrow">NETWORK</div><h2>Workers</h2></div><span className="count">{workers.length}</span></div>
          {workers.length === 0 ? <p className="empty">No workers registered yet. Start the local worker agent to join this coordinator.</p> : (
            <div className="table">
              {workers.map((worker) => {
                const online = Date.now() - new Date(worker.last_seen_at).getTime() < 30_000;
                return <div className="worker-row" key={worker.id}>
                  <span className={online ? "dot" : "dot offline"} />
                  <div className="worker-name">
                    <div className="worker-title">
                      <strong>{worker.name || "Unnamed worker"}</strong>
                      <span className={worker.kind === "inference" ? "worker-kind inference" : "worker-kind"}>{worker.kind === "inference" ? "INFERENCE" : "COMPUTE"}</span>
                    </div>
                    <small>{worker.cpu_cores} CPU cores · {worker.memory_mb.toLocaleString()} MB advertised</small>
                    {worker.kind === "inference" && (
                      <small className="worker-runtime">
                        {worker.runtime || "Runtime unknown"} · {worker.model || "Model unknown"}
                      </small>
                    )}
                    {worker.kind === "inference" && worker.operations && worker.operations.length > 0 && (
                      <div className="operation-list">{worker.operations.map((operation) => <span key={operation}>{operation}</span>)}</div>
                    )}
                    {worker.kind === "inference" && worker.inference_endpoint && (
                      <small className="worker-endpoint" title={worker.inference_endpoint}>{worker.inference_endpoint}</small>
                    )}
                  </div>
                  <span className="state">{online ? "Active" : "Stale"}</span>
                </div>;
              })}
            </div>
          )}
        </article>
      </section>

      <section className="panel section" style={{ marginBottom: 18 }}>
        <div className="section-heading"><div><div className="eyebrow">DATA PLANE · V1 PREVIEW</div><h2>JSONL datasets</h2></div><span className="count">{datasets.length} uploaded this session</span></div>
        <form onSubmit={uploadDataset} className="job-form">
          <label>Choose a JSONL file<input name="dataset" type="file" accept=".jsonl,.ndjson,application/x-ndjson,application/json" required /></label>
          <button type="submit" disabled={uploading}>{uploading ? "Uploading and validating…" : "Upload dataset ↗"}</button>
        </form>
        <p className="hint">Uploads are stored on the coordinator disk, checksummed, validated, and split at JSONL line boundaries. Current development limit: 256 MiB.</p>
        {datasets.length > 0 && <div className="job-list">{datasets.map((dataset) => <div className="job-row" key={dataset.id}>
          <div className="job-meta"><strong>{dataset.filename}</strong><span className="pill completed">{dataset.status}</span></div>
          <div className="job-details"><span>{dataset.record_count.toLocaleString()} records</span><span>{(dataset.size_bytes / 1048576).toFixed(2)} MiB</span><span>{dataset.partitions} partitions</span></div>
          <small>SHA-256 · {dataset.checksum}</small>
          <small>Dataset ID · {dataset.id}</small>
        </div>)}</div>}
      </section>

      <section className="panel section jobs-section">
        <div className="section-heading"><div><div className="eyebrow">EXECUTION QUEUE</div><h2>Recent jobs</h2></div><span className="muted">{lastUpdated ? `Updated ${lastUpdated.toLocaleTimeString()}` : "Waiting for data"}</span></div>
        {jobs.length === 0 ? <p className="empty">No jobs yet. Submit a workload to see batches appear here.</p> : (
          <div className="job-list">
            {[...jobs].reverse().map((job) => {
              const done = job.batches.filter((batch) => batch.status === "completed").length;
              const percent = job.batches.length ? Math.round((done / job.batches.length) * 100) : 0;
              return <div className="job-row" key={job.id}>
                <div className="job-meta"><strong>{job.id.slice(0, 12)}…</strong><span className={`pill ${job.status}`}>{job.status}</span></div>
                <div className="job-details"><span>{job.total_items.toLocaleString()} records</span><span>{job.batches.length} batches · {job.batch_size.toLocaleString()} / batch</span><span>{done}/{job.batches.length} complete</span></div>
                <div className="progress"><span style={{ width: `${percent}%` }} /></div>
              </div>;
            })}
          </div>
        )}
      </section>

      <footer><span>OPENCOMPUTE FABRIC</span><span>V1 preview · local data plane</span></footer>
    </main>
  );
}
