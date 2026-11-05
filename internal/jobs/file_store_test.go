package jobs

import (
	"path/filepath"
	"testing"
	"time"
)

func TestFileStorePersistsJobsWorkersAndLeases(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "ocf.json")
	first, err := OpenFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	job, err := first.Create(20, 10)
	if err != nil {
		t.Fatal(err)
	}
	worker, err := first.RegisterWorker("persistent-worker", 2, 2048)
	if err != nil {
		t.Fatal(err)
	}
	lease, err := first.LeaseNext(worker.ID, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	second, err := OpenFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := second.Get(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Batches[0].LeaseID != lease.ID || restored.Batches[0].FencingToken != lease.FencingToken {
		t.Fatalf("lease state not restored: %+v", restored.Batches[0])
	}
	completed, err := second.CompleteLease(worker.ID, lease.ID, lease.FencingToken, BatchResult{Processed: 10, Checksum: 77})
	if err != nil {
		t.Fatal(err)
	}
	if completed.Batches[0].Status != BatchCompleted {
		t.Fatal("completion not persisted")
	}
	third, err := OpenFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	final, err := third.Get(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if final.Batches[0].Result == nil || final.Batches[0].Result.Checksum != 77 {
		t.Fatalf("result not restored: %+v", final.Batches[0])
	}
}

func TestFileStorePersistsInferenceCapabilities(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "ocf.json")
	first, err := OpenFileStore(path)
	if err != nil { t.Fatal(err) }
	created, err := first.RegisterInferenceWorker("qwen-node", 2, 4096, "llama.cpp", "qwen2.5-0.5b-instruct-q4_k_m.gguf", "http://127.0.0.1:8081", []string{"chat-completions", "completion"})
	if err != nil { t.Fatal(err) }
	second, err := OpenFileStore(path)
	if err != nil { t.Fatal(err) }
	workers := second.ListWorkers()
	if len(workers) != 1 { t.Fatalf("got %d workers, want 1", len(workers)) }
	got := workers[0]
	if got.ID != created.ID || got.Kind != "inference" || got.Runtime != "llama.cpp" || got.Model != created.Model || got.InferenceEndpoint != created.InferenceEndpoint || len(got.Operations) != 2 {
		t.Fatalf("inference capabilities not restored: %+v", got)
	}
}
