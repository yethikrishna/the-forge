// Package snapshot provides time-travel filesystem snapshots.
// Capture, browse, diff, and restore project states.
package snapshot

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Type is the snapshot type.
type Type string

const (
	TypeManual       Type = "manual"
	TypeAuto         Type = "auto"
	TypePreOperation Type = "pre-operation"
	TypePostOperation Type = "post-operation"
	TypeMilestone    Type = "milestone"
)

// FileEntry records a single file's state in a snapshot.
type FileEntry struct {
	Path    string `json:"path"`
	SHA256  string `json:"sha256"`
	Size    int64  `json:"size"`
	ModTime string `json:"mod_time"`
	IsDir   bool   `json:"is_dir"`
}

// Snapshot is a point-in-time capture of a directory tree.
type Snapshot struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Type        Type        `json:"type"`
	Description string      `json:"description"`
	RootDir     string      `json:"root_dir"`
	Files       []FileEntry `json:"files"`
	FileCount   int         `json:"file_count"`
	TotalSize   int64       `json:"total_size"`
	GitBranch   string      `json:"git_branch,omitempty"`
	GitCommit   string      `json:"git_commit,omitempty"`
	GitDirty    bool        `json:"git_dirty,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
	Tags        []string    `json:"tags,omitempty"`
}

// DiffResult describes a difference between two snapshots.
type DiffResult struct {
	Added    []string `json:"added"`
	Removed  []string `json:"removed"`
	Modified []string `json:"modified"`
	Unchanged int     `json:"unchanged"`
}

// Stats holds snapshot store statistics.
type Stats struct {
	TotalSnapshots int            `json:"total_snapshots"`
	TotalFiles     int            `json:"total_files"`
	TotalSize      int64          `json:"total_size"`
	ByType         map[Type]int   `json:"by_type"`
}

// Store manages snapshots.
type Store struct {
	dir       string
	snapshots map[string]*Snapshot
	mu        sync.RWMutex
}

// NewStore creates a new snapshot store.
func NewStore(dir string) (*Store, error) {
	os.MkdirAll(dir, 0755)
	s := &Store{
		dir:       dir,
		snapshots: make(map[string]*Snapshot),
	}
	s.load()
	return s, nil
}

// IgnorePatterns returns default ignore patterns.
func IgnorePatterns() []string {
	return []string{
		".git", ".svn", ".hg",
		"node_modules", "vendor",
		"__pycache__", ".pytest_cache",
		"bin", "dist", "out", "build",
		".DS_Store", "Thumbs.db",
	}
}

// Create creates a new snapshot of the given directory.
func (s *Store) Create(name string, snapType Type, rootDir, description string) (*Snapshot, error) {
	ignoreSet := make(map[string]bool)
	for _, p := range IgnorePatterns() {
		ignoreSet[p] = true
	}

	var files []FileEntry
	var totalSize int64

	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		rel, err := filepath.Rel(rootDir, path)
		if err != nil {
			return nil
		}

		for _, part := range strings.Split(rel, string(filepath.Separator)) {
			if ignoreSet[part] {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		entry := FileEntry{
			Path:    rel,
			Size:    info.Size(),
			ModTime: info.ModTime().Format(time.RFC3339),
			IsDir:   d.IsDir(),
		}

		if !d.IsDir() {
			data, err := os.ReadFile(path)
			if err == nil {
				h := sha256.Sum256(data)
				entry.SHA256 = fmt.Sprintf("%x", h[:])
			}
			totalSize += info.Size()
		}

		files = append(files, entry)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking directory: %w", err)
	}

	snap := &Snapshot{
		ID:          fmt.Sprintf("snap-%d", time.Now().UnixNano()),
		Name:        name,
		Type:        snapType,
		Description: description,
		RootDir:     rootDir,
		Files:       files,
		FileCount:   len(files),
		TotalSize:   totalSize,
		CreatedAt:   time.Now(),
	}

	s.mu.Lock()
	s.snapshots[snap.ID] = snap
	s.save()
	s.mu.Unlock()

	return snap, nil
}

// Get returns a snapshot by ID.
func (s *Store) Get(id string) (*Snapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snap, ok := s.snapshots[id]
	if !ok {
		return nil, false
	}
	copy := *snap
	return &copy, true
}

// List returns snapshots, optionally filtered by type.
func (s *Store) List(typeFilter string) []Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]Snapshot, 0, len(s.snapshots))
	for _, snap := range s.snapshots {
		if typeFilter != "" && string(snap.Type) != typeFilter {
			continue
		}
		result = append(result, *snap)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}

// Delete removes a snapshot.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.snapshots[id]; !ok {
		return fmt.Errorf("snapshot %q not found", id)
	}
	delete(s.snapshots, id)
	s.save()
	return nil
}

// Compare compares two snapshots.
func (s *Store) Compare(oldID, newID string) (*DiffResult, error) {
	s.mu.RLock()
	oldSnap, ok1 := s.snapshots[oldID]
	newSnap, ok2 := s.snapshots[newID]
	s.mu.RUnlock()

	if !ok1 {
		return nil, fmt.Errorf("snapshot %q not found", oldID)
	}
	if !ok2 {
		return nil, fmt.Errorf("snapshot %q not found", newID)
	}

	oldFiles := make(map[string]FileEntry)
	for _, f := range oldSnap.Files {
		oldFiles[f.Path] = f
	}

	newFiles := make(map[string]FileEntry)
	for _, f := range newSnap.Files {
		newFiles[f.Path] = f
	}

	result := &DiffResult{}

	for path, newEntry := range newFiles {
		if oldEntry, ok := oldFiles[path]; ok {
			if !oldEntry.IsDir && !newEntry.IsDir {
				if oldEntry.SHA256 != newEntry.SHA256 {
					result.Modified = append(result.Modified, path)
				} else {
					result.Unchanged++
				}
			}
		} else if !newEntry.IsDir {
			result.Added = append(result.Added, path)
		}
	}

	for path, oldEntry := range oldFiles {
		if _, ok := newFiles[path]; !ok && !oldEntry.IsDir {
			result.Removed = append(result.Removed, path)
		}
	}

	return result, nil
}

// Stats returns snapshot store statistics.
func (s *Store) Stats() Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := Stats{
		ByType: make(map[Type]int),
	}
	for _, snap := range s.snapshots {
		stats.TotalSnapshots++
		stats.TotalFiles += snap.FileCount
		stats.TotalSize += snap.TotalSize
		stats.ByType[snap.Type]++
	}
	return stats
}

func (s *Store) save() {
	if s.dir == "" {
		return
	}
	os.MkdirAll(s.dir, 0755)
	data, _ := json.MarshalIndent(s.snapshots, "", "  ")
	os.WriteFile(filepath.Join(s.dir, "snapshots.json"), data, 0644)
}

func (s *Store) load() {
	if s.dir == "" {
		return
	}
	data, err := os.ReadFile(filepath.Join(s.dir, "snapshots.json"))
	if err != nil {
		return
	}
	json.Unmarshal(data, &s.snapshots)
}
