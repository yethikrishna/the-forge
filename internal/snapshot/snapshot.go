// Package snapshot provides time-travel filesystem snapshots.
// Capture, browse, diff, and restore project states without git.
// Each snapshot records file hashes and metadata, enabling instant
// comparison and selective restoration.
//
// Every moment, preserved.
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
	Description string      `json:"description"`
	RootDir     string      `json:"root_dir"`
	Files       []FileEntry `json:"files"`
	FileCount   int         `json:"file_count"`
	TotalSize   int64       `json:"total_size"`
	CreatedAt   time.Time   `json:"created_at"`
	Tags        []string    `json:"tags,omitempty"`
}

// DiffResult describes a difference between two snapshots.
type DiffResult struct {
	Added    []FileEntry `json:"added"`
	Removed  []FileEntry `json:"removed"`
	Modified []FileEntry `json:"modified"`
	Unchanged int        `json:"unchanged"`
}

// Manager manages snapshots.
type Manager struct {
	dir       string
	snapshots map[string]*Snapshot
	mu        sync.RWMutex
}

// NewManager creates a new snapshot manager.
func NewManager(dir string) *Manager {
	os.MkdirAll(dir, 0755)
	m := &Manager{
		dir:       dir,
		snapshots: make(map[string]*Snapshot),
	}
	m.load()
	return m
}

// IgnorePatterns returns default ignore patterns.
func IgnorePatterns() []string {
	return []string{
		".git", ".svn", ".hg",
		"node_modules", "vendor",
		"__pycache__", ".pytest_cache",
		"bin", "dist", "out", "build",
		".DS_Store", "Thumbs.db",
		"*.pyc", "*.pyo",
		"*.exe", "*.dll", "*.so", "*.dylib",
	}
}

// Capture creates a new snapshot of the given directory.
func (m *Manager) Capture(rootDir, name, description string, ignorePatterns []string) (*Snapshot, error) {
	ignoreSet := make(map[string]bool)
	for _, p := range ignorePatterns {
		ignoreSet[p] = true
	}

	var files []FileEntry
	var totalSize int64

	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip errors
		}

		rel, err := filepath.Rel(rootDir, path)
		if err != nil {
			return nil
		}

		// Check ignore patterns
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
		Description: description,
		RootDir:     rootDir,
		Files:       files,
		FileCount:   len(files),
		TotalSize:   totalSize,
		CreatedAt:   time.Now(),
	}

	m.mu.Lock()
	m.snapshots[snap.ID] = snap
	m.save()
	m.mu.Unlock()

	return snap, nil
}

// Get returns a snapshot by ID.
func (m *Manager) Get(id string) (*Snapshot, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.snapshots[id]
	if !ok {
		return nil, false
	}
	copy := *s
	return &copy, true
}

// List returns all snapshots.
func (m *Manager) List() []Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Snapshot, 0, len(m.snapshots))
	for _, s := range m.snapshots {
		result = append(result, *s)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}

// Delete removes a snapshot.
func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.snapshots[id]; !ok {
		return fmt.Errorf("snapshot %q not found", id)
	}
	delete(m.snapshots, id)
	m.save()
	return nil
}

// Diff compares two snapshots.
func (m *Manager) Diff(oldID, newID string) (*DiffResult, error) {
	m.mu.RLock()
	oldSnap, ok1 := m.snapshots[oldID]
	newSnap, ok2 := m.snapshots[newID]
	m.mu.RUnlock()

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

	// Find added and modified
	for path, newEntry := range newFiles {
		if oldEntry, ok := oldFiles[path]; ok {
			if oldEntry.SHA256 != newEntry.SHA256 && !oldEntry.IsDir && !newEntry.IsDir {
				result.Modified = append(result.Modified, newEntry)
			} else if !oldEntry.IsDir && !newEntry.IsDir {
				result.Unchanged++
			}
		} else if !newEntry.IsDir {
			result.Added = append(result.Added, newEntry)
		}
	}

	// Find removed
	for path, oldEntry := range oldFiles {
		if _, ok := newFiles[path]; !ok && !oldEntry.IsDir {
			result.Removed = append(result.Removed, oldEntry)
		}
	}

	return result, nil
}

// RestoreFiles restores specific files from a snapshot.
func (m *Manager) RestoreFiles(snapID, rootDir string, filePaths []string) error {
	m.mu.RLock()
	snap, ok := m.snapshots[snapID]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("snapshot %q not found", snapID)
	}

	fileSet := make(map[string]bool)
	for _, p := range filePaths {
		fileSet[p] = true
	}

	for _, entry := range snap.Files {
		if !fileSet[entry.Path] || entry.IsDir {
			continue
		}

		srcPath := filepath.Join(snap.RootDir, entry.Path)
		dstPath := filepath.Join(rootDir, entry.Path)

		data, err := os.ReadFile(srcPath)
		if err != nil {
			return fmt.Errorf("reading %s: %w", entry.Path, err)
		}

		os.MkdirAll(filepath.Dir(dstPath), 0755)
		if err := os.WriteFile(dstPath, data, 0644); err != nil {
			return fmt.Errorf("writing %s: %w", entry.Path, err)
		}
	}

	return nil
}

// Stats returns snapshot manager statistics.
func (m *Manager) Stats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var totalFiles int
	var totalSize int64
	for _, s := range m.snapshots {
		totalFiles += s.FileCount
		totalSize += s.TotalSize
	}

	return map[string]interface{}{
		"total_snapshots": len(m.snapshots),
		"total_files":     totalFiles,
		"total_size":      totalSize,
	}
}

// RenderSnapshot renders a snapshot for display.
func RenderSnapshot(s *Snapshot) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Snapshot: %s\n", s.Name)
	fmt.Fprintf(&b, "ID: %s\n", s.ID)
	fmt.Fprintf(&b, "Root: %s\n", s.RootDir)
	fmt.Fprintf(&b, "Description: %s\n", s.Description)
	fmt.Fprintf(&b, "Files: %d\n", s.FileCount)
	fmt.Fprintf(&b, "Size: %s\n", formatSize(s.TotalSize))
	fmt.Fprintf(&b, "Created: %s\n", s.CreatedAt.Format(time.RFC3339))
	if len(s.Tags) > 0 {
		fmt.Fprintf(&b, "Tags: %s\n", strings.Join(s.Tags, ", "))
	}

	return b.String()
}

// RenderDiff renders a diff result for display.
func RenderDiff(d *DiffResult) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Added: %d files\n", len(d.Added))
	for _, f := range d.Added {
		fmt.Fprintf(&b, "  + %s (%s)\n", f.Path, formatSize(f.Size))
	}

	fmt.Fprintf(&b, "Removed: %d files\n", len(d.Removed))
	for _, f := range d.Removed {
		fmt.Fprintf(&b, "  - %s (%s)\n", f.Path, formatSize(f.Size))
	}

	fmt.Fprintf(&b, "Modified: %d files\n", len(d.Modified))
	for _, f := range d.Modified {
		fmt.Fprintf(&b, "  ~ %s (%s)\n", f.Path, formatSize(f.Size))
	}

	fmt.Fprintf(&b, "Unchanged: %d files\n", d.Unchanged)

	return b.String()
}

func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func (m *Manager) save() {
	if m.dir == "" {
		return
	}
	os.MkdirAll(m.dir, 0755)
	data, _ := json.MarshalIndent(m.snapshots, "", "  ")
	os.WriteFile(filepath.Join(m.dir, "snapshots.json"), data, 0644)
}

func (m *Manager) load() {
	if m.dir == "" {
		return
	}
	data, err := os.ReadFile(filepath.Join(m.dir, "snapshots.json"))
	if err != nil {
		return
	}
	json.Unmarshal(data, &m.snapshots)
}
