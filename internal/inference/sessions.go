package inference

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

var ErrSessionNotFound = errors.New("inference session not found")

type Session struct {
	ID             string    `json:"id"`
	WorkerID       string    `json:"worker_id"`
	Epoch          uint64    `json:"epoch"`
	State          string    `json:"state"`
	Reassignments  uint64    `json:"reassignments"`
	CreatedAt      time.Time `json:"created_at"`
	LastActivityAt time.Time `json:"last_activity_at"`
}

type SessionManager struct {
	mu       sync.RWMutex
	sessions map[string]Session
}

func NewSessionManager() *SessionManager {
	return &SessionManager{sessions: make(map[string]Session)}
}

func (m *SessionManager) Create(workerID string) (Session, error) {
	idBytes := make([]byte, 16)
	if _, err := rand.Read(idBytes); err != nil {
		return Session{}, err
	}
	now := time.Now().UTC()
	session := Session{
		ID: idString(idBytes), WorkerID: workerID, Epoch: 1,
		State: "active", CreatedAt: now, LastActivityAt: now,
	}
	m.mu.Lock()
	m.sessions[session.ID] = session
	m.mu.Unlock()
	return session, nil
}

func (m *SessionManager) Get(id string) (Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	session, ok := m.sessions[id]
	if !ok {
		return Session{}, ErrSessionNotFound
	}
	return session, nil
}

func (m *SessionManager) Reassign(id, workerID string) (Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	session, ok := m.sessions[id]
	if !ok {
		return Session{}, ErrSessionNotFound
	}
	session.WorkerID = workerID
	session.Epoch++
	session.Reassignments++
	session.State = "active"
	session.LastActivityAt = time.Now().UTC()
	m.sessions[id] = session
	return session, nil
}

func (m *SessionManager) Touch(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	session, ok := m.sessions[id]
	if !ok {
		return ErrSessionNotFound
	}
	session.LastActivityAt = time.Now().UTC()
	m.sessions[id] = session
	return nil
}

func (m *SessionManager) Invalidate(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	session, ok := m.sessions[id]
	if !ok {
		return
	}
	session.State = "worker_unavailable"
	m.sessions[id] = session
}

func (m *SessionManager) InvalidateWorker(workerID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, session := range m.sessions {
		if session.WorkerID == workerID {
			session.State = "worker_unavailable"
			m.sessions[id] = session
		}
	}
}

func idString(value []byte) string {
	return hex.EncodeToString(value)
}
