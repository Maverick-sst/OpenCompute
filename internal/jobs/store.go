package jobs

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/Maverick-sst/OpenCompute/internal/batch"
)

var ErrNotFound = errors.New("job not found")

type Status string

const (
	StatusQueued Status = "queued"
)

type Job struct {
	ID         string        `json:"id"`
	TotalItems int64         `json:"total_items"`
	BatchSize  int64         `json:"batch_size"`
	Status     Status        `json:"status"`
	Batches    []batch.Range `json:"batches"`
	CreatedAt  time.Time     `json:"created_at"`
}

type Store interface {
	Create(totalItems, batchSize int64) (Job, error)
	Get(id string) (Job, error)
}

type MemoryStore struct {
	mu   sync.RWMutex
	jobs map[string]Job
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{jobs: make(map[string]Job)}
}

func (s *MemoryStore) Create(totalItems, batchSize int64) (Job, error) {
	ranges, err := batch.Partition(totalItems, batchSize)
	if err != nil {
		return Job{}, err
	}
	idBytes := make([]byte, 16)
	if _, err := rand.Read(idBytes); err != nil {
		return Job{}, err
	}
	job := Job{
		ID: hex.EncodeToString(idBytes), TotalItems: totalItems,
		BatchSize: batchSize, Status: StatusQueued,
		Batches: ranges, CreatedAt: time.Now().UTC(),
	}
	s.mu.Lock()
	s.jobs[job.ID] = job
	s.mu.Unlock()
	return job, nil
}

func (s *MemoryStore) Get(id string) (Job, error) {
	s.mu.RLock()
	job, ok := s.jobs[id]
	s.mu.RUnlock()
	if !ok {
		return Job{}, ErrNotFound
	}
	return job, nil
}
