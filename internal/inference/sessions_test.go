package inference

import "testing"

func TestSessionCreateAndReassignAdvancesEpoch(t *testing.T) {
	manager := NewSessionManager()
	created, err := manager.Create("worker-a")
	if err != nil {
		t.Fatal(err)
	}
	if created.WorkerID != "worker-a" || created.Epoch != 1 || created.State != "active" {
		t.Fatalf("unexpected initial session: %+v", created)
	}
	reassigned, err := manager.Reassign(created.ID, "worker-b")
	if err != nil {
		t.Fatal(err)
	}
	if reassigned.WorkerID != "worker-b" || reassigned.Epoch != 2 || reassigned.Reassignments != 1 {
		t.Fatalf("unexpected reassigned session: %+v", reassigned)
	}
}

func TestSessionInvalidationAndTouch(t *testing.T) {
	manager := NewSessionManager()
	created, err := manager.Create("worker-a")
	if err != nil {
		t.Fatal(err)
	}
	manager.InvalidateWorker("worker-a")
	invalid, err := manager.Get(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if invalid.State != "worker_unavailable" {
		t.Fatalf("state=%q want worker_unavailable", invalid.State)
	}
	if err := manager.Touch(created.ID); err != nil {
		t.Fatal(err)
	}
}

func TestSessionGetUnknown(t *testing.T) {
	manager := NewSessionManager()
	if _, err := manager.Get("missing"); err != ErrSessionNotFound {
		t.Fatalf("error=%v want %v", err, ErrSessionNotFound)
	}
}
