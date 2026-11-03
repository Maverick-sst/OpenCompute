package jobs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type diskState struct {
	Jobs    map[string]*Job   `json:"jobs"`
	Workers map[string]Worker `json:"workers"`
	Leases  map[string]Lease  `json:"leases"`
	Fence   uint64            `json:"fence"`
}

// FileStore persists coordinator state to one local JSON file. It is suitable
// for a single coordinator process on a persistent disk, not multi-replica use.
type FileStore struct {
	mu     sync.Mutex
	path   string
	memory *MemoryStore
}

func OpenFileStore(path string) (*FileStore, error) {
	if path == "" {
		return nil, fmt.Errorf("state file path is required")
	}
	s := &FileStore{path: path, memory: NewMemoryStore()}
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("read state file: %w", err)
		}
		return s, nil
	}
	var state diskState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("decode state file: %w", err)
	}
	if state.Jobs == nil {
		state.Jobs = map[string]*Job{}
	}
	if state.Workers == nil {
		state.Workers = map[string]Worker{}
	}
	if state.Leases == nil {
		state.Leases = map[string]Lease{}
	}
	s.memory.mu.Lock()
	s.memory.jobs = state.Jobs
	s.memory.workers = state.Workers
	s.memory.leases = state.Leases
	s.memory.fence = state.Fence
	s.memory.mu.Unlock()
	return s, nil
}

func (s *FileStore) persist() error {
	s.memory.mu.Lock()
	state := diskState{Jobs: make(map[string]*Job, len(s.memory.jobs)), Workers: make(map[string]Worker, len(s.memory.workers)), Leases: make(map[string]Lease, len(s.memory.leases)), Fence: s.memory.fence}
	for id, j := range s.memory.jobs {
		copy := cloneJob(j)
		state.Jobs[id] = &copy
	}
	for id, w := range s.memory.workers {
		state.Workers[id] = w
	}
	for id, l := range s.memory.leases {
		state.Leases[id] = l
	}
	s.memory.mu.Unlock()

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".ocf-state-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary state file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		return fmt.Errorf("replace state file: %w", err)
	}
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}

func (s *FileStore) Create(total, size int64) (Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, err := s.memory.Create(total, size)
	if err != nil {
		return Job{}, err
	}
	if err := s.persist(); err != nil {
		return Job{}, err
	}
	return job, nil
}
func (s *FileStore) Get(id string) (Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.memory.Get(id)
}
func (s *FileStore) RegisterWorker(name string, cores int, mem int64) (Worker, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.memory.RegisterWorker(name, cores, mem)
	if err != nil {
		return Worker{}, err
	}
	if err := s.persist(); err != nil {
		return Worker{}, err
	}
	return w, nil
}
func (s *FileStore) Heartbeat(id string) (Worker, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.memory.Heartbeat(id)
	if err != nil {
		return Worker{}, err
	}
	if err := s.persist(); err != nil {
		return Worker{}, err
	}
	return w, nil
}
func (s *FileStore) LeaseNext(id string, d time.Duration) (Lease, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, err := s.memory.LeaseNext(id, d)
	if err != nil {
		return Lease{}, err
	}
	if err := s.persist(); err != nil {
		return Lease{}, err
	}
	return l, nil
}
func (s *FileStore) CompleteLease(workerID, leaseID string, token uint64, result BatchResult) (Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, err := s.memory.CompleteLease(workerID, leaseID, token, result)
	if err != nil {
		return Job{}, err
	}
	if err := s.persist(); err != nil {
		return Job{}, err
	}
	return job, nil
}

func (s *FileStore) ListJobs() []Job {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.memory.ListJobs()
}

func (s *FileStore) ListWorkers() []Worker {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.memory.ListWorkers()
}
