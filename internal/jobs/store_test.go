package jobs

import (
	"errors"
	"testing"
	"time"
)

func TestWorkerLeaseCompletionLifecycle(t *testing.T) {
	s := NewMemoryStore()
	job, err := s.Create(100, 50)
	if err != nil {
		t.Fatal(err)
	}
	w, err := s.RegisterWorker("worker-a", 2, 2048)
	if err != nil {
		t.Fatal(err)
	}
	first, err := s.LeaseNext(w.ID, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if first.Batch.Start != 0 || first.Batch.End != 50 {
		t.Fatalf("unexpected lease: %+v", first)
	}
	updated, err := s.CompleteLease(w.ID, first.ID, first.FencingToken, BatchResult{Processed: 50, Checksum: 11})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != JobRunning {
		t.Fatalf("status=%s want running", updated.Status)
	}
	second, err := s.LeaseNext(w.ID, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	updated, err = s.CompleteLease(w.ID, second.ID, second.FencingToken, BatchResult{Processed: 50, Checksum: 22})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != JobCompleted {
		t.Fatalf("status=%s want completed", updated.Status)
	}
	if updated.Batches[1].Result == nil || updated.Batches[1].Result.Checksum != 22 {
		t.Fatal("result was not stored")
	}
	if _, err = s.Get(job.ID); err != nil {
		t.Fatal(err)
	}
}
func TestLeaseOwnershipAndFencing(t *testing.T) {
	s := NewMemoryStore()
	_, _ = s.Create(10, 10)
	a, _ := s.RegisterWorker("a", 1, 512)
	b, _ := s.RegisterWorker("b", 1, 512)
	l, err := s.LeaseNext(a.ID, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.CompleteLease(b.ID, l.ID, l.FencingToken, BatchResult{Processed: 10}); !errors.Is(err, ErrLeaseOwnership) {
		t.Fatalf("got %v", err)
	}
	if _, err = s.CompleteLease(a.ID, l.ID, l.FencingToken+1, BatchResult{Processed: 10}); !errors.Is(err, ErrLeaseOwnership) {
		t.Fatalf("got %v", err)
	}
}
func TestExpiredLeaseIsReassigned(t *testing.T) {
	s := NewMemoryStore()
	_, _ = s.Create(10, 10)
	a, _ := s.RegisterWorker("a", 1, 512)
	b, _ := s.RegisterWorker("b", 1, 512)
	first, err := s.LeaseNext(a.ID, time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	second, err := s.LeaseNext(b.ID, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == second.ID || second.FencingToken <= first.FencingToken {
		t.Fatalf("reassignment did not fence old lease: first=%+v second=%+v", first, second)
	}
	if _, err = s.CompleteLease(a.ID, first.ID, first.FencingToken, BatchResult{Processed: 10}); !errors.Is(err, ErrLeaseNotFound) {
		t.Fatalf("stale completion got %v", err)
	}
}
func TestCompletionRequiresExactProcessedCount(t *testing.T) {
	s := NewMemoryStore()
	_, _ = s.Create(10, 10)
	w, _ := s.RegisterWorker("a", 1, 512)
	l, _ := s.LeaseNext(w.ID, time.Minute)
	if _, err := s.CompleteLease(w.ID, l.ID, l.FencingToken, BatchResult{Processed: 9}); err == nil {
		t.Fatal("expected processed count validation error")
	}
}
