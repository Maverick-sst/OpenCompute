import { useCallback, useEffect, useMemo, useState, type FormEvent } from "react";

type Job = {
  id: string;
  total_items: number;
  batch_size: number;
  status: string;
  batches: { status: string; start: number; end: number }[];
  created_at: string;
};

type Worker = {
  id: string;
  name: string;
  cpu_cores: number;
  memory_mb: number;
  last_seen_at: string;
};

const API_DEFAULT = import.meta.env.VITE_API_BASE_URL || "http://127.0.0.1:8080";

export default function App() {
  const [apiBase, setApiBase] = useState(API_DEFAULT);
  const [token, setToken] = useState("");
  const [jobs, setJobs] = useState<Job[]>([]);
  const [workers, setWorkers] = useState<Worker[]>([]);
  const [totalItems, setTotalItems] = useState("1000000");
  const [batchSize, setBatchSize] = useState("100000");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null);

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
  const completedBatches = jobs.reduce(
    (sum, job) => sum + job.batches.filter((batch) => batch.status === "completed").length,
    0,
  );
  const totalBatches = jobs.reduce((sum, job) => sum + job.batches.length, 0);

  return (
    <main className="shell">
      <header className="topbar">
        <div>
          <div className="eyebrow">OPEN COMPUTE FABRIC / V0</div>
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
        <article className="stat panel"><span>Registered workers</span><strong>{workers.length}</strong><small>{activeWorkers} seen in the last 30 seconds</small></article>
        <article className="stat panel"><span>Jobs submitted</span><strong>{jobs.length}</strong><small>Across this coordinator</small></article>
        <article className="stat panel"><span>Batches completed</span><strong>{completedBatches}<em> / {totalBatches}</em></strong><small>Completed batch leases</small></article>
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
          <p className="hint">Start with synthetic data. V0 workers process deterministic item ranges, not arbitrary user code.</p>
        </article>

        <article className="panel section">
          <div className="section-heading"><div><div className="eyebrow">NETWORK</div><h2>Workers</h2></div><span className="count">{workers.length}</span></div>
          {workers.length === 0 ? <p className="empty">No workers registered yet. Start the local worker agent to join this coordinator.</p> : (
            <div className="table">
              {workers.map((worker) => {
                const online = Date.now() - new Date(worker.last_seen_at).getTime() < 30_000;
                return <div className="worker-row" key={worker.id}>
                  <span className={online ? "dot" : "dot offline"} />
                  <div className="worker-name"><strong>{worker.name || "Unnamed worker"}</strong><small>{worker.cpu_cores} CPU cores · {worker.memory_mb} MB advertised</small></div>
                  <span className="state">{online ? "Active" : "Idle"}</span>
                </div>;
              })}
            </div>
          )}
        </article>
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

      <footer><span>OPENCOMPUTE FABRIC</span><span>V0 · experimental network</span></footer>
    </main>
  );
}
