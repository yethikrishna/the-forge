// Package watcher provides file system change detection.
// It watches directories for file changes and emits events
// for created, modified, deleted, and renamed files.
//
// The forge sees all that changes.
package watcher

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// EventType represents the kind of file change.
type EventType int

const (
	EventCreate EventType = iota
	EventModify
	EventDelete
	EventRename
)

func (e EventType) String() string {
	switch e {
	case EventCreate:
		return "CREATE"
	case EventModify:
		return "MODIFY"
	case EventDelete:
		return "DELETE"
	case EventRename:
		return "RENAME"
	default:
		return "UNKNOWN"
	}
}

// Event represents a single file change event.
type Event struct {
	Type      EventType
	Path      string
	Timestamp time.Time
}

// String returns a human-readable event description.
func (e Event) String() string {
	return fmt.Sprintf("[%s] %s", e.Type, e.Path)
}

// Handler is called when a file change is detected.
type Handler func(Event)

// Config configures the file watcher.
type Config struct {
	// Directories to watch (recursively).
	Paths []string

	// File extensions to include (e.g. ".go", ".py"). Empty = all.
	Extensions []string

	// Patterns to ignore (glob syntax, e.g. ".git", "node_modules", "*.tmp").
	Ignore []string

	// Debounce window — coalesce rapid changes into one event.
	Debounce time.Duration

	// Polling interval for checking changes.
	PollInterval time.Duration
}

// DefaultConfig returns a sensible default configuration.
func DefaultConfig(paths ...string) Config {
	return Config{
		Paths:         paths,
		Extensions:    nil, // all files
		Ignore:        []string{".git", "node_modules", ".forge", "vendor", "*.tmp", "*.swp", ".DS_Store"},
		Debounce:      300 * time.Millisecond,
		PollInterval:  500 * time.Millisecond,
	}
}

// Watcher monitors file system changes.
type Watcher struct {
	config  Config
	handler Handler

	mu       sync.Mutex
	snapshot map[string]os.FileInfo // path -> last known file info
	stopCh   chan struct{}
	running  bool
}

// New creates a new file watcher.
func New(config Config, handler Handler) *Watcher {
	if config.PollInterval == 0 {
		config.PollInterval = 500 * time.Millisecond
	}
	if config.Debounce == 0 {
		config.Debounce = 300 * time.Millisecond
	}
	return &Watcher{
		config:   config,
		handler:  handler,
		snapshot: make(map[string]os.FileInfo),
		stopCh:   make(chan struct{}),
	}
}

// Start begins watching. Blocks until Stop is called.
func (w *Watcher) Start() error {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return fmt.Errorf("watcher already running")
	}
	w.running = true
	w.mu.Unlock()

	// Build initial snapshot
	if err := w.buildSnapshot(); err != nil {
		return fmt.Errorf("initial scan failed: %w", err)
	}

	// Debounce buffer
	debounced := make(map[string]Event)
	var debounceTimer *time.Timer

	flush := func() {
		w.mu.Lock()
		for _, evt := range debounced {
			w.handler(evt)
		}
		debounced = make(map[string]Event)
		debounceTimer = nil
		w.mu.Unlock()
	}

	ticker := time.NewTicker(w.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopCh:
			if debounceTimer != nil {
				debounceTimer.Stop()
				flush()
			}
			w.mu.Lock()
			w.running = false
			w.mu.Unlock()
			return nil

		case <-ticker.C:
			changes := w.detectChanges()
			if len(changes) > 0 {
				w.mu.Lock()
				for _, evt := range changes {
					debounced[evt.Path] = evt // last event wins
				}
				if debounceTimer != nil {
					debounceTimer.Reset(w.config.Debounce)
				} else {
					debounceTimer = time.AfterFunc(w.config.Debounce, flush)
				}
				w.mu.Unlock()
			}

		case <-time.After(w.config.Debounce):
			// fallback flush (shouldn't normally hit this)
		}
	}
}

// Stop stops the watcher.
func (w *Watcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.running {
		close(w.stopCh)
		w.stopCh = make(chan struct{})
	}
}

// IsRunning reports whether the watcher is active.
func (w *Watcher) IsRunning() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.running
}

// Snapshot returns the current file snapshot (for testing/inspection).
func (w *Watcher) Snapshot() map[string]os.FileInfo {
	w.mu.Lock()
	defer w.mu.Unlock()
	result := make(map[string]os.FileInfo, len(w.snapshot))
	for k, v := range w.snapshot {
		result[k] = v
	}
	return result
}

// buildSnapshot walks all configured paths and records file metadata.
func (w *Watcher) buildSnapshot() error {
	w.snapshot = make(map[string]os.FileInfo)
	for _, root := range w.config.Paths {
		if err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // skip unreadable files
			}
			if info.IsDir() {
				if w.shouldIgnoreDir(path) {
					return filepath.SkipDir
				}
				return nil
			}
			if w.shouldIncludeFile(path) {
				w.snapshot[path] = info
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

// detectChanges compares current filesystem state to the snapshot.
func (w *Watcher) detectChanges() []Event {
	var events []Event
	now := time.Now()

	for _, root := range w.config.Paths {
		filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				if w.shouldIgnoreDir(path) {
					return filepath.SkipDir
				}
				return nil
			}
			if !w.shouldIncludeFile(path) {
				return nil
			}

			prev, exists := w.snapshot[path]
			if !exists {
				// New file
				w.snapshot[path] = info
				events = append(events, Event{
					Type:      EventCreate,
					Path:      path,
					Timestamp: now,
				})
			} else if info.ModTime().After(prev.ModTime()) || info.Size() != prev.Size() {
				// Modified file
				w.snapshot[path] = info
				events = append(events, Event{
					Type:      EventModify,
					Path:      path,
					Timestamp: now,
				})
			}

			return nil
		})
	}

	// Check for deleted files
	w.mu.Lock()
	for path := range w.snapshot {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			delete(w.snapshot, path)
			events = append(events, Event{
				Type:      EventDelete,
				Path:      path,
				Timestamp: now,
			})
		}
	}
	w.mu.Unlock()

	return events
}

// shouldIgnoreDir checks if a directory should be skipped.
func (w *Watcher) shouldIgnoreDir(path string) bool {
	base := filepath.Base(path)
	for _, pattern := range w.config.Ignore {
		if matchPattern(base, pattern) {
			return true
		}
	}
	return false
}

// shouldIncludeFile checks if a file matches the extension filter.
func (w *Watcher) shouldIncludeFile(path string) bool {
	if len(w.config.Extensions) == 0 {
		return true
	}
	ext := strings.ToLower(filepath.Ext(path))
	for _, allowed := range w.config.Extensions {
		if strings.EqualFold(ext, allowed) {
			return true
		}
	}
	return false
}

// matchPattern does simple glob matching.
func matchPattern(name, pattern string) bool {
	if pattern == "" {
		return false
	}
	// Exact match
	if name == pattern {
		return true
	}
	// Wildcard suffix (*.tmp, *.swp)
	if strings.HasPrefix(pattern, "*.") {
		suffix := pattern[1:] // ".tmp"
		return strings.HasSuffix(name, suffix)
	}
	// Prefix match (e.g. ".git" matches ".git", ".github")
	if strings.HasPrefix(name, pattern) {
		return true
	}
	return false
}
