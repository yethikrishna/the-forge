// Package snapshot provides environment checkpointing for the forge.
// Before every great swing, mark where you stand.
package snapshot

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Snapshot is a checkpoint of the environment state.
type Snapshot struct {
	ID          string            `json:"id"`
	Label       string            `json:"label"`
	CreatedAt   time.Time         `json:"created_at"`
	GitCommit   string            `json:"git_commit"`
	GitBranch   string            `json:"git_branch"`
	GitDirty    bool              `json:"git_dirty"`
	GitStash    string            `json:"git_stash,omitempty"`
	Files       map[string]string `json:"files"`
	EnvVars     map[string]string `json:"env_vars,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	ParentID    string            `json:"parent_id,omitempty"`
	Notes       string            `json:"notes,omitempty"`
	Size        int64             `json:"size"`
}

// Store manages snapshots on disk.
type Store struct {
	dir string
}

// NewStore creates a snapshot store.
func NewStore(dir string) *Store {
	return &Store{dir: dir}
}

// Create captures the current environment state as a snapshot.
func (s *Store) Create(label string, opts ...CreateOption) (*Snapshot, error) {
	cfg := &createConfig{
		includeGit:  true,
		includeEnv:  false,
		includeFiles: true,
		maxFileSize: 64 * 1024, // 64KB per file
	}
	for _, o := range opts {
		o(cfg)
	}

	snap := &Snapshot{
		ID:        generateID(),
		Label:     label,
		CreatedAt: time.Now().UTC(),
		Files:     make(map[string]string),
		EnvVars:   make(map[string]string),
	}

	// Capture git state
	if cfg.includeGit {
		snap.GitCommit = gitOutput("rev-parse", "HEAD")
		snap.GitBranch = gitOutput("rev-parse", "--abbrev-ref", "HEAD")
		snap.GitDirty = gitOutput("status", "--porcelain") != ""
		snap.GitStash = gitOutput("stash", "list")
	}

	// Capture environment variables
	if cfg.includeEnv && len(cfg.envKeys) > 0 {
		for _, key := range cfg.envKeys {
			if val, ok := os.LookupEnv(key); ok {
				snap.EnvVars[key] = val
			}
		}
	}

	// Capture files
	if cfg.includeFiles {
		var totalSize int64
		for _, pattern := range cfg.filePatterns {
			if pattern == "" {
				pattern = "."
			}
			matches, err := filepath.Glob(pattern)
			if err != nil {
				continue
			}
			for _, path := range matches {
				info, err := os.Stat(path)
				if err != nil || info.IsDir() {
					continue
				}
				if info.Size() > int64(cfg.maxFileSize) {
					continue
				}
				data, err := os.ReadFile(path)
				if err != nil {
					continue
				}
				snap.Files[path] = string(data)
				totalSize += info.Size()
			}
		}
		snap.Size = totalSize
	}

	snap.Tags = cfg.tags
	snap.Notes = cfg.notes

	// Save to disk
	if err := s.save(snap); err != nil {
		return nil, fmt.Errorf("snapshot save: %w", err)
	}

	return snap, nil
}

// List returns all snapshots, newest first.
func (s *Store) List() ([]*Snapshot, error) {
	os.MkdirAll(s.dir, 0o755)

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("read dir: %w", err)
	}

	var snaps []*Snapshot
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(s.dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var snap Snapshot
		if err := json.Unmarshal(data, &snap); err != nil {
			continue
		}
		snaps = append(snaps, &snap)
	}

	sort.Slice(snaps, func(i, j int) bool {
		return snaps[i].CreatedAt.After(snaps[j].CreatedAt)
	})

	return snaps, nil
}

// Get retrieves a snapshot by ID.
func (s *Store) Get(id string) (*Snapshot, error) {
	path := filepath.Join(s.dir, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("snapshot not found: %s", id)
	}

	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("invalid snapshot: %w", err)
	}

	return &snap, nil
}

// Delete removes a snapshot.
func (s *Store) Delete(id string) error {
	path := filepath.Join(s.dir, id+".json")
	return os.Remove(path)
}

// Restore restores files from a snapshot.
func (s *Store) Restore(id string, opts ...RestoreOption) error {
	snap, err := s.Get(id)
	if err != nil {
		return err
	}

	cfg := &restoreConfig{
		overwrite: false,
		restoreGit: false,
	}
	for _, o := range opts {
		o(cfg)
	}

	// Restore files
	for path, content := range snap.Files {
		if !cfg.overwrite {
			if _, err := os.Stat(path); err == nil {
				continue // skip existing
			}
		}
		dir := filepath.Dir(path)
		os.MkdirAll(dir, 0o755)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return fmt.Errorf("restore %s: %w", path, err)
		}
	}

	// Restore git state
	if cfg.restoreGit && snap.GitCommit != "" {
		exec.Command("git", "checkout", snap.GitCommit).Run()
	}

	return nil
}

// Diff compares two snapshots and returns the differences.
func (s *Store) Diff(id1, id2 string) (map[string]FileDiff, error) {
	snap1, err := s.Get(id1)
	if err != nil {
		return nil, err
	}
	snap2, err := s.Get(id2)
	if err != nil {
		return nil, err
	}

	diffs := make(map[string]FileDiff)
	allFiles := make(map[string]bool)

	for path := range snap1.Files {
		allFiles[path] = true
	}
	for path := range snap2.Files {
		allFiles[path] = true
	}

	for path := range allFiles {
		content1, ok1 := snap1.Files[path]
		content2, ok2 := snap2.Files[path]

		switch {
		case ok1 && !ok2:
			diffs[path] = FileDiff{Status: "deleted", OldContent: content1}
		case !ok1 && ok2:
			diffs[path] = FileDiff{Status: "added", NewContent: content2}
		case ok1 && ok2 && content1 != content2:
			diffs[path] = FileDiff{Status: "modified", OldContent: content1, NewContent: content2}
		}
	}

	return diffs, nil
}

// FileDiff represents a file difference between snapshots.
type FileDiff struct {
	Status    string `json:"status"` // added, deleted, modified
	OldContent string `json:"old_content,omitempty"`
	NewContent string `json:"new_content,omitempty"`
}

// createConfig for snapshot creation.
type createConfig struct {
	includeGit   bool
	includeEnv   bool
	includeFiles bool
	maxFileSize  int
	envKeys      []string
	filePatterns []string
	tags         []string
	notes        string
}

// CreateOption customizes snapshot creation.
type CreateOption func(*createConfig)

// WithGit includes git state in the snapshot.
func WithGit(v bool) CreateOption {
	return func(c *createConfig) { c.includeGit = v }
}

// WithEnv includes specified environment variables.
func WithEnv(keys ...string) CreateOption {
	return func(c *createConfig) {
		c.includeEnv = true
		c.envKeys = keys
	}
}

// WithFiles includes files matching patterns.
func WithFiles(patterns ...string) CreateOption {
	return func(c *createConfig) {
		c.includeFiles = true
		c.filePatterns = patterns
	}
}

// WithTags adds tags to the snapshot.
func WithTags(tags ...string) CreateOption {
	return func(c *createConfig) { c.tags = tags }
}

// WithNotes adds notes to the snapshot.
func WithNotes(notes string) CreateOption {
	return func(c *createConfig) { c.notes = notes }
}

// WithParent links to a parent snapshot.
func WithParent(parentID string) CreateOption {
	return func(c *createConfig) {} // handled separately
}

// restoreConfig for snapshot restoration.
type restoreConfig struct {
	overwrite  bool
	restoreGit bool
}

// RestoreOption customizes snapshot restoration.
type RestoreOption func(*restoreConfig)

// WithOverwrite allows overwriting existing files.
func WithOverwrite(v bool) RestoreOption {
	return func(c *restoreConfig) { c.overwrite = v }
}

// WithGitRestore also restores git state.
func WithGitRestore(v bool) RestoreOption {
	return func(c *restoreConfig) { c.restoreGit = v }
}

func (s *Store) save(snap *Snapshot) error {
	os.MkdirAll(s.dir, 0o755)

	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	path := filepath.Join(s.dir, snap.ID+".json")
	return os.WriteFile(path, data, 0o644)
}

func generateID() string {
	return fmt.Sprintf("snap-%d", time.Now().UnixNano())
}

func gitOutput(args ...string) string {
	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
