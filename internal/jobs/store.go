package jobs

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"github.com/Maverick-sst/OpenCompute/internal/batch"
	"sync"
	"time"
)

var (
	ErrNotFound       = errors.New("resource not found")
	ErrLeaseNotFound  = errors.New("lease not found")
	ErrLeaseOwnership = errors.New("lease ownership or fencing token mismatch")
	ErrLeaseExpired   = errors.New("lease expired")
)

type JobStatus string
type BatchStatus string

const (
	JobQueued      JobStatus   = "queued"
	JobRunning     JobStatus   = "running"
	JobCompleted   JobStatus   = "completed"
	BatchQueued    BatchStatus = "queued"
	BatchLeased    BatchStatus = "leased"
	BatchCompleted BatchStatus = "completed"
)

type BatchResult struct {
	Processed   int64     `json:"processed"`
	Checksum    uint64    `json:"checksum"`
	CompletedAt time.Time `json:"completed_at"`
}
type BatchTask struct {
	batch.Range
	Status       BatchStatus  `json:"status"`
	LeaseID      string       `json:"lease_id,omitempty"`
	WorkerID     string       `json:"worker_id,omitempty"`
	FencingToken uint64       `json:"fencing_token,omitempty"`
	LeaseExpires time.Time    `json:"lease_expires_at,omitempty"`
	Result       *BatchResult `json:"result,omitempty"`
}
type Job struct {
	ID         string      `json:"id"`
	TotalItems int64       `json:"total_items"`
	BatchSize  int64       `json:"batch_size"`
	Status     JobStatus   `json:"status"`
	Batches    []BatchTask `json:"batches"`
	CreatedAt  time.Time   `json:"created_at"`
}
type Worker struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	CPUCores     int       `json:"cpu_cores"`
	MemoryMB     int64     `json:"memory_mb"`
	RegisteredAt time.Time `json:"registered_at"`
	LastSeenAt   time.Time `json:"last_seen_at"`
}
type Lease struct {
	ID           string      `json:"id"`
	JobID        string      `json:"job_id"`
	WorkerID     string      `json:"worker_id"`
	Batch        batch.Range `json:"batch"`
	FencingToken uint64      `json:"fencing_token"`
	ExpiresAt    time.Time   `json:"expires_at"`
}
type Store interface {
	Create(int64, int64) (Job, error)
	Get(string) (Job, error)
	RegisterWorker(string, int, int64) (Worker, error)
	Heartbeat(string) (Worker, error)
	LeaseNext(string, time.Duration) (Lease, error)
	CompleteLease(string, string, uint64, BatchResult) (Job, error)
	ListJobs() []Job
	ListWorkers() []Worker
}
type MemoryStore struct {
	mu      sync.Mutex
	jobs    map[string]*Job
	workers map[string]Worker
	leases  map[string]Lease
	fence   uint64
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{jobs: map[string]*Job{}, workers: map[string]Worker{}, leases: map[string]Lease{}}
}
func newID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
func (s *MemoryStore) Create(total, size int64) (Job, error) {
	ranges, err := batch.Partition(total, size)
	if err != nil {
		return Job{}, err
	}
	id, err := newID()
	if err != nil {
		return Job{}, err
	}
	tasks := make([]BatchTask, len(ranges))
	for i, r := range ranges {
		tasks[i] = BatchTask{Range: r, Status: BatchQueued}
	}
	j := &Job{ID: id, TotalItems: total, BatchSize: size, Status: JobQueued, Batches: tasks, CreatedAt: time.Now().UTC()}
	s.mu.Lock()
	s.jobs[id] = j
	out := cloneJob(j)
	s.mu.Unlock()
	return out, nil
}
func (s *MemoryStore) Get(id string) (Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	j, ok := s.jobs[id]
	if !ok {
		return Job{}, ErrNotFound
	}
	return cloneJob(j), nil
}
func (s *MemoryStore) RegisterWorker(name string, cores int, mem int64) (Worker, error) {
	if cores < 1 || mem < 1 {
		return Worker{}, fmt.Errorf("cpu_cores and memory_mb must be positive")
	}
	id, err := newID()
	if err != nil {
		return Worker{}, err
	}
	now := time.Now().UTC()
	w := Worker{ID: id, Name: name, CPUCores: cores, MemoryMB: mem, RegisteredAt: now, LastSeenAt: now}
	s.mu.Lock()
	s.workers[id] = w
	s.mu.Unlock()
	return w, nil
}
func (s *MemoryStore) Heartbeat(id string) (Worker, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, ok := s.workers[id]
	if !ok {
		return Worker{}, ErrNotFound
	}
	w.LastSeenAt = time.Now().UTC()
	s.workers[id] = w
	return w, nil
}
func (s *MemoryStore) LeaseNext(workerID string, d time.Duration) (Lease, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	worker, ok := s.workers[workerID]
	if !ok {
		return Lease{}, ErrNotFound
	}
	now := time.Now().UTC()
	worker.LastSeenAt = now
	s.workers[workerID] = worker

	// Requeue expired leases before assigning new work.
	for _, job := range s.jobs {
		for i := range job.Batches {
			task := &job.Batches[i]
			if task.Status == BatchLeased && !task.LeaseExpires.After(now) {
				delete(s.leases, task.LeaseID)
				task.Status = BatchQueued
				task.LeaseID = ""
				task.WorkerID = ""
				task.FencingToken = 0
				task.LeaseExpires = time.Time{}
			}
		}
	}

	for _, job := range s.jobs {
		for i := range job.Batches {
			task := &job.Batches[i]
			if task.Status != BatchQueued {
				continue
			}
			leaseID, err := newID()
			if err != nil {
				return Lease{}, err
			}
			s.fence++
			expires := now.Add(d)
			task.Status = BatchLeased
			task.LeaseID = leaseID
			task.WorkerID = workerID
			task.FencingToken = s.fence
			task.LeaseExpires = expires

			lease := Lease{
				ID: leaseID, JobID: job.ID, WorkerID: workerID,
				Batch: task.Range, FencingToken: s.fence, ExpiresAt: expires,
			}
			s.leases[leaseID] = lease
			job.Status = JobRunning
			return lease, nil
		}
	}
	return Lease{}, ErrLeaseNotFound
}

func (s *MemoryStore) CompleteLease(workerID, leaseID string, token uint64, result BatchResult) (Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	lease, ok := s.leases[leaseID]
	if !ok {
		return Job{}, ErrLeaseNotFound
	}
	if lease.WorkerID != workerID || lease.FencingToken != token {
		return Job{}, ErrLeaseOwnership
	}
	now := time.Now().UTC()
	if !lease.ExpiresAt.After(now) {
		return Job{}, ErrLeaseExpired
	}
	job, ok := s.jobs[lease.JobID]
	if !ok {
		return Job{}, ErrNotFound
	}
	for i := range job.Batches {
		task := &job.Batches[i]
		if task.LeaseID != leaseID || task.FencingToken != token {
			continue
		}
		if result.Processed != task.End-task.Start {
			return Job{}, fmt.Errorf("processed count must equal batch length")
		}
		result.CompletedAt = now
		task.Status = BatchCompleted
		task.Result = &result
		delete(s.leases, leaseID)

		allDone := true
		for _, other := range job.Batches {
			if other.Status != BatchCompleted {
				allDone = false
				break
			}
		}
		if allDone {
			job.Status = JobCompleted
		}
		return cloneJob(job), nil
	}
	return Job{}, ErrLeaseNotFound
}

func (s *MemoryStore) ListJobs() []Job {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		out = append(out, cloneJob(job))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out
}

func (s *MemoryStore) ListWorkers() []Worker {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Worker, 0, len(s.workers))
	for _, worker := range s.workers {
		out = append(out, worker)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].RegisteredAt.Equal(out[j].RegisteredAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].RegisteredAt.Before(out[j].RegisteredAt)
	})
	return out
}

func cloneJob(j *Job) Job {
	c := *j
	c.Batches = append([]BatchTask(nil), j.Batches...)
	for i := range c.Batches {
		if j.Batches[i].Result != nil {
			r := *j.Batches[i].Result
			c.Batches[i].Result = &r
		}
	}
	return c
}
