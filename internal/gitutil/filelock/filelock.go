// Package filelock provides advisory file locking for concurrent agent operations.
// Prevents data corruption when multiple agents write to the same files.
//
// Lock before you write.
package filelock

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// LockType represents the type of file lock.
type LockType string

const (
	LockShared    LockType = "shared"    // Multiple readers
	LockExclusive LockType = "exclusive" // Single writer
)

// LockInfo represents metadata about a held lock.
type LockInfo struct {
	Path       string    `json:"path"`
	LockType   LockType  `json:"lock_type"`
	AgentID    string    `json:"agent_id"`
	SessionID  string    `json:"session_id"`
	AcquiredAt time.Time `json:"acquired_at"`
	PID        int       `json:"pid"`
}

// Lock represents a file lock.
type Lock struct {
	file     *os.File
	info     LockInfo
	dir      string
	released bool
}

// Manager manages file locks for concurrent agent operations.
type Manager struct {
	mu      sync.Mutex
	dir     string
	locks   map[string]*Lock // path → lock
	maxWait time.Duration
}

// NewManager creates a file lock manager.
func NewManager(dir string) *Manager {
	return &Manager{
		dir:     dir,
		locks:   make(map[string]*Lock),
		maxWait: 30 * time.Second,
	}
}

// SetMaxWait sets the maximum time to wait for a lock.
func (m *Manager) SetMaxWait(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.maxWait = d
}

// Acquire attempts to acquire a file lock. Blocks until acquired or timeout.
func (m *Manager) Acquire(filePath string, lockType LockType, agentID, sessionID string) (*Lock, error) {
	m.mu.Lock()
	maxWait := m.maxWait
	m.mu.Unlock()

	// Check if already locked by us
	if existing, ok := m.locks[filePath]; ok {
		if existing.info.AgentID == agentID {
			return existing, nil
		}
	}

	// Ensure lock directory exists
	lockDir := m.dir
	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		return nil, fmt.Errorf("filelock: create dir: %w", err)
	}

	lockPath := filepath.Join(lockDir, filepath.Base(filePath)+".lock")

	// Try to acquire with timeout
	deadline := time.Now().Add(maxWait)
	for {
		file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
		if err != nil {
			return nil, fmt.Errorf("filelock: open: %w", err)
		}

		how := syscall.LOCK_EX
		if lockType == LockShared {
			how = syscall.LOCK_SH
		}

		// Non-blocking try
		err = syscall.Flock(int(file.Fd()), how|syscall.LOCK_NB)
		if err == nil {
			// Acquired!
			lock := &Lock{
				file: file,
				info: LockInfo{
					Path:       filePath,
					LockType:   lockType,
					AgentID:    agentID,
					SessionID:  sessionID,
					AcquiredAt: time.Now(),
					PID:        os.Getpid(),
				},
				dir: lockDir,
			}

			// Write lock metadata
			metadata, _ := json.Marshal(lock.info)
			file.Truncate(0)
			file.Seek(0, 0)
			file.Write(metadata)

			m.mu.Lock()
			m.locks[filePath] = lock
			m.mu.Unlock()

			return lock, nil
		}

		file.Close()

		if time.Now().After(deadline) {
			// Read who holds the lock
			holder := m.readHolder(lockPath)
			return nil, fmt.Errorf("filelock: timeout acquiring %s on %s (held by %s)", lockType, filePath, holder)
		}

		time.Sleep(100 * time.Millisecond)
	}
}

// TryAcquire attempts to acquire a lock without blocking.
func (m *Manager) TryAcquire(filePath string, lockType LockType, agentID, sessionID string) (*Lock, error) {
	m.mu.Lock()
	m.maxWait = 0
	oldMaxWait := m.maxWait
	m.mu.Unlock()

	lock, err := m.Acquire(filePath, lockType, agentID, sessionID)

	m.mu.Lock()
	m.maxWait = oldMaxWait
	m.mu.Unlock()

	return lock, err
}

// Release releases a file lock.
func (l *Lock) Release() error {
	if l.released {
		return nil
	}

	l.released = true

	if l.file != nil {
		syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
		l.file.Close()
		os.Remove(l.file.Name())
	}

	return nil
}

// Info returns the lock's metadata.
func (l *Lock) Info() LockInfo {
	return l.info
}

// IsHeld checks if a file is currently locked.
func (m *Manager) IsHeld(filePath string) bool {
	m.mu.Lock()
	_, ok := m.locks[filePath]
	m.mu.Unlock()
	return ok
}

// ListHeld returns all currently held locks.
func (m *Manager) ListHeld() []LockInfo {
	m.mu.Lock()
	defer m.mu.Unlock()

	infos := make([]LockInfo, 0, len(m.locks))
	for _, l := range m.locks {
		infos = append(infos, l.info)
	}
	return infos
}

// ReleaseAll releases all held locks.
func (m *Manager) ReleaseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, l := range m.locks {
		l.Release()
	}
	m.locks = make(map[string]*Lock)
}

// ReleaseByAgent releases all locks held by a specific agent.
func (m *Manager) ReleaseByAgent(agentID string) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for path, l := range m.locks {
		if l.info.AgentID == agentID {
			l.Release()
			delete(m.locks, path)
			count++
		}
	}
	return count
}

// readHolder reads who holds a lock from the lock file.
func (m *Manager) readHolder(lockPath string) string {
	data, err := os.ReadFile(lockPath)
	if err != nil {
		return "unknown"
	}

	var info LockInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return "unknown"
	}

	return fmt.Sprintf("%s (pid %d, since %s)", info.AgentID, info.PID,
		info.AcquiredAt.Format("15:04:05"))
}

// FormatLockInfo renders lock info for display.
func FormatLockInfo(info LockInfo) string {
	return fmt.Sprintf("%-10s %-40s agent:%s session:%s since:%s",
		info.LockType, info.Path, info.AgentID, info.SessionID,
		info.AcquiredAt.Format("15:04:05"))
}
