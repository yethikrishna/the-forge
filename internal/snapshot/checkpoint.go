// Package snapshot provides environment checkpoint capabilities.
// Capture full workspace state: files, git status, environment variables.
// Name snapshots, restore them, diff between them.
//
// Git only tracks committed changes. Agents make uncommitted changes.
// Snapshots fill the gap.
package snapshot

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Status represents the status of a snapshot.
type Status string

const (
	StatusActive   Status = "active"
	StatusRestored Status = "restored"
	StatusDeleted  Status = "deleted"
)

// Checkpoint captures a complete environment state at a point in time.
type Checkpoint struct {
	ID          string            `json:"id"`
	Name        string            `json:"name,omitempty"`
	Timestamp   time.Time         `json:"timestamp"`
	Status      Status            `json:"status"`
	WorkDir     string            `json:"work_dir"`
	GitBranch   string            `json:"git_branch,omitempty"`
	GitCommit   string            `json:"git_commit,omitempty"`
	GitDirty    bool              `json:"git_dirty"`
	GitStaged   bool              `json:"git_staged"`
	EnvVars     map[string]string `json:"env_vars,omitempty"`
	FileCount   int               `json:"file_count"`
	TotalSize   int64             `json:"total_size"`
	Checksum    string            `json:"checksum"`
	Tags        []string          `json:"tags,omitempty"`
	Description string            `json:"description,omitempty"`
	Agent       string            `json:"agent,omitempty"`
	Session     string            `json:"session,omitempty"`
}

// FileEntry records metadata about a single file in the snapshot.
type FileEntry struct {
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	Mode     uint32 `json:"mode"`
	Modified string `json:"modified"`
	Checksum string `json:"checksum"`
}

// Manifest describes the full contents of a snapshot archive.
type Manifest struct {
	Checkpoint Checkpoint  `json:"checkpoint"`
	Files      []FileEntry `json:"files"`
	GitDiff    string      `json:"git_diff,omitempty"`
	GitStatus  string      `json:"git_status,omitempty"`
}

// Store manages snapshot storage at a given directory.
type Store struct {
	Dir     string
	WorkDir string
}

// NewStore creates a snapshot store rooted at dir, with workDir as the
// working directory to snapshot.
func NewStore(dir, workDir string) *Store {
	return &Store{Dir: dir, WorkDir: workDir}
}

// Create captures the current environment state into a named snapshot.
// It records git state, environment variables, and archives tracked files.
func (s *Store) Create(name string, opts ...CreateOption) (*Checkpoint, error) {
	cfg := defaultCreateConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create snapshot dir: %w", err)
	}

	cp := &Checkpoint{
		ID:        generateID(name),
		Name:      name,
		Timestamp: time.Now(),
		Status:    StatusActive,
		WorkDir:   s.WorkDir,
		Tags:      cfg.tags,
		Agent:     cfg.agent,
		Session:   cfg.session,
	}

	// Capture git state
	if err := s.captureGitState(cp); err != nil {
		// Git capture is best-effort; don't fail the whole snapshot
		cp.GitBranch = "(unknown)"
	}

	// Capture environment variables
	if cfg.captureEnv {
		cp.EnvVars = captureEnvVars()
	}

	// Walk the working directory, collecting file metadata
	manifest := Manifest{Checkpoint: *cp}
	var totalSize int64
	var fileCount int

	// Build a list of files to include
	files, err := s.collectFiles(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to collect files: %w", err)
	}

	hasher := sha256.New()
	for _, f := range files {
		info, err := os.Stat(f)
		if err != nil {
			continue
		}
		if info.IsDir() {
			continue
		}

		totalSize += info.Size()
		fileCount++

		// Compute checksum
		fh, err := os.Open(f)
		if err != nil {
			continue
		}
		fileHash := sha256.New()
		io.Copy(fileHash, fh)
		fh.Close()

		relPath, _ := filepath.Rel(s.WorkDir, f)
		entry := FileEntry{
			Path:     relPath,
			Size:     info.Size(),
			Mode:     uint32(info.Mode()),
			Modified: info.ModTime().Format(time.RFC3339),
			Checksum: fmt.Sprintf("%x", fileHash.Sum(nil)),
		}
		manifest.Files = append(manifest.Files, entry)
		hasher.Write([]byte(entry.Checksum))
	}

	cp.FileCount = fileCount
	cp.TotalSize = totalSize
	cp.Checksum = fmt.Sprintf("%x", hasher.Sum(nil))

	// Capture git diff and status
	manifest.GitDiff = s.gitDiff()
	manifest.GitStatus = s.gitStatus()
	manifest.Checkpoint = *cp

	// Write the archive
	if err := s.writeArchive(cp.ID, &manifest, files); err != nil {
		return nil, fmt.Errorf("failed to write snapshot archive: %w", err)
	}

	// Write the checkpoint metadata
	if err := s.writeCheckpoint(cp); err != nil {
		return nil, fmt.Errorf("failed to write checkpoint: %w", err)
	}

	return cp, nil
}

// List returns all snapshots sorted by timestamp (newest first).
func (s *Store) List() ([]*Checkpoint, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var checkpoints []*Checkpoint
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.Dir, e.Name()))
		if err != nil {
			continue
		}
		var cp Checkpoint
		if err := json.Unmarshal(data, &cp); err != nil {
			continue
		}
		checkpoints = append(checkpoints, &cp)
	}

	sort.Slice(checkpoints, func(i, k int) bool {
		return checkpoints[i].Timestamp.After(checkpoints[k].Timestamp)
	})

	return checkpoints, nil
}

// Get retrieves a snapshot by ID or name.
func (s *Store) Get(idOrName string) (*Checkpoint, error) {
	checkpoints, err := s.List()
	if err != nil {
		return nil, err
	}

	for _, cp := range checkpoints {
		if cp.ID == idOrName || cp.Name == idOrName {
			return cp, nil
		}
	}
	return nil, fmt.Errorf("snapshot %q not found", idOrName)
}

// Restore reverts the working directory to a previous snapshot state.
func (s *Store) Restore(idOrName string) (*Checkpoint, error) {
	cp, err := s.Get(idOrName)
	if err != nil {
		return nil, err
	}

	archivePath := filepath.Join(s.Dir, cp.ID+".tar.gz")
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("snapshot archive not found: %s", archivePath)
	}

	// Restore from archive
	if err := s.extractArchive(archivePath, s.WorkDir); err != nil {
		return nil, fmt.Errorf("failed to extract snapshot: %w", err)
	}

	// Update checkpoint status
	cp.Status = StatusRestored
	if err := s.writeCheckpoint(cp); err != nil {
		return nil, fmt.Errorf("failed to update checkpoint: %w", err)
	}

	return cp, nil
}

// Delete removes a snapshot and its archive.
func (s *Store) Delete(idOrName string) error {
	cp, err := s.Get(idOrName)
	if err != nil {
		return err
	}

	// Delete archive
	archivePath := filepath.Join(s.Dir, cp.ID+".tar.gz")
	os.Remove(archivePath)

	// Delete metadata
	metaPath := filepath.Join(s.Dir, cp.ID+".json")
	if err := os.Remove(metaPath); err != nil {
		return fmt.Errorf("failed to delete snapshot metadata: %w", err)
	}

	return nil
}

// Diff compares two snapshots and returns a human-readable diff.
func (s *Store) Diff(idA, idB string) (string, error) {
	cpA, err := s.Get(idA)
	if err != nil {
		return "", fmt.Errorf("snapshot A: %w", err)
	}
	cpB, err := s.Get(idB)
	if err != nil {
		return "", fmt.Errorf("snapshot B: %w", err)
	}

	manifestA, err := s.loadManifest(cpA.ID)
	if err != nil {
		return "", fmt.Errorf("load manifest A: %w", err)
	}
	manifestB, err := s.loadManifest(cpB.ID)
	if err != nil {
		return "", fmt.Errorf("load manifest B: %w", err)
	}

	return diffManifests(manifestA, manifestB, cpA, cpB), nil
}

// --- internal helpers ---

type createConfig struct {
	tags        []string
	agent       string
	session     string
	captureEnv  bool
	excludeDirs []string
}

func defaultCreateConfig() createConfig {
	return createConfig{
		captureEnv:  true,
		excludeDirs: []string{".git", "node_modules", ".forge", "__pycache__", "vendor", ".svn"},
	}
}

// CreateOption configures snapshot creation.
type CreateOption func(*createConfig)

// WithTags adds tags to the snapshot.
func WithTags(tags []string) CreateOption {
	return func(c *createConfig) { c.tags = tags }
}

// WithAgent records which agent triggered the snapshot.
func WithAgent(agent string) CreateOption {
	return func(c *createConfig) { c.agent = agent }
}

// WithSession records the session ID.
func WithSession(session string) CreateOption {
	return func(c *createConfig) { c.session = session }
}

// WithCaptureEnv controls whether environment variables are captured.
func WithCaptureEnv(capture bool) CreateOption {
	return func(c *createConfig) { c.captureEnv = capture }
}

// WithExcludeDirs sets directories to exclude from the snapshot.
func WithExcludeDirs(dirs []string) CreateOption {
	return func(c *createConfig) { c.excludeDirs = dirs }
}

func generateID(name string) string {
	base := "snap"
	if name != "" {
		base = sanitizeName(name)
	}
	return fmt.Sprintf("%s-%d", base, time.Now().UnixNano())
}

func sanitizeName(name string) string {
	s := strings.ToLower(name)
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")
	result := make([]byte, 0, len(s))
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' {
			result = append(result, byte(c))
		}
	}
	r := string(result)
	if len(r) > 32 {
		r = r[:32]
	}
	return r
}

func (s *Store) captureGitState(cp *Checkpoint) error {
	// Branch
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = s.WorkDir
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("git branch: %w", err)
	}
	cp.GitBranch = strings.TrimSpace(string(out))

	// Commit
	cmd = exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = s.WorkDir
	out, err = cmd.Output()
	if err != nil {
		return fmt.Errorf("git commit: %w", err)
	}
	cp.GitCommit = strings.TrimSpace(string(out))[:12]

	// Dirty check
	cmd = exec.Command("git", "diff", "--quiet")
	cmd.Dir = s.WorkDir
	cp.GitDirty = cmd.Run() != nil

	// Staged check
	cmd = exec.Command("git", "diff", "--cached", "--quiet")
	cmd.Dir = s.WorkDir
	cp.GitStaged = cmd.Run() != nil

	return nil
}

func (s *Store) gitDiff() string {
	cmd := exec.Command("git", "diff", "HEAD")
	cmd.Dir = s.WorkDir
	out, _ := cmd.Output()
	return string(out)
}

func (s *Store) gitStatus() string {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = s.WorkDir
	out, _ := cmd.Output()
	return string(out)
}

func captureEnvVars() map[string]string {
	env := make(map[string]string)
	for _, e := range os.Environ() {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			key := parts[0]
			// Skip sensitive env vars
			if strings.Contains(strings.ToUpper(key), "KEY") ||
				strings.Contains(strings.ToUpper(key), "SECRET") ||
				strings.Contains(strings.ToUpper(key), "TOKEN") ||
				strings.Contains(strings.ToUpper(key), "PASSWORD") ||
				strings.Contains(strings.ToUpper(key), "CREDENTIAL") {
				env[key] = "••••••••"
				continue
			}
			env[key] = parts[1]
		}
	}
	return env
}

func (s *Store) collectFiles(cfg createConfig) ([]string, error) {
	var files []string
	exclude := make(map[string]bool)
	for _, d := range cfg.excludeDirs {
		exclude[d] = true
	}

	err := filepath.Walk(s.WorkDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		rel, err := filepath.Rel(s.WorkDir, path)
		if err != nil {
			return nil
		}

		// Skip excluded directories
		if info.IsDir() {
			parts := strings.Split(rel, string(filepath.Separator))
			for _, p := range parts {
				if exclude[p] {
					return filepath.SkipDir
				}
			}
			return nil
		}

		// Skip the snapshot store itself
		if strings.HasPrefix(path, s.Dir) {
			return nil
		}

		files = append(files, path)
		return nil
	})

	return files, err
}

func (s *Store) writeArchive(id string, manifest *Manifest, files []string) error {
	archivePath := filepath.Join(s.Dir, id+".tar.gz")
	f, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	// Write manifest
	manifestData, _ := json.MarshalIndent(manifest, "", "  ")
	hdr := &tar.Header{
		Name:    "manifest.json",
		Size:    int64(len(manifestData)),
		Mode:    0o644,
		ModTime: time.Now(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	if _, err := tw.Write(manifestData); err != nil {
		return err
	}

	// Write files
	for _, path := range files {
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			continue
		}

		relPath, err := filepath.Rel(s.WorkDir, path)
		if err != nil {
			continue
		}

		// Try to create a proper tar header
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			header = &tar.Header{
				Name:    relPath,
				Size:    info.Size(),
				Mode:    int64(info.Mode()),
				ModTime: info.ModTime(),
			}
		}
		header.Name = relPath

		if err := tw.WriteHeader(header); err != nil {
			continue
		}

		fh, err := os.Open(path)
		if err != nil {
			continue
		}
		io.Copy(tw, fh)
		fh.Close()
	}

	return nil
}

func (s *Store) extractArchive(archivePath, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Skip the manifest
		if hdr.Name == "manifest.json" {
			continue
		}

		target := filepath.Join(destDir, hdr.Name)

		switch hdr.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(target, os.FileMode(hdr.Mode))
		case tar.TypeReg:
			os.MkdirAll(filepath.Dir(target), 0o755)
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				continue
			}
			io.Copy(out, tr)
			out.Close()
		}
	}

	return nil
}

func (s *Store) writeCheckpoint(cp *Checkpoint) error {
	data, err := json.MarshalIndent(cp, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.Dir, cp.ID+".json"), data, 0o644)
}

func (s *Store) loadManifest(id string) (*Manifest, error) {
	archivePath := filepath.Join(s.Dir, id+".tar.gz")
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if hdr.Name == "manifest.json" {
			data, err := io.ReadAll(tr)
			if err != nil {
				return nil, err
			}
			var m Manifest
			if err := json.Unmarshal(data, &m); err != nil {
				return nil, err
			}
			return &m, nil
		}
	}

	return nil, fmt.Errorf("manifest not found in archive")
}

func diffManifests(a, b *Manifest, cpA, cpB *Checkpoint) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Snapshot Diff: %s ↔ %s\n", cpA.NameOrID(), cpB.NameOrID()))
	sb.WriteString(fmt.Sprintf("  %s → %s\n\n", cpA.Timestamp.Format(time.RFC3339), cpB.Timestamp.Format(time.RFC3339)))

	// Build file maps
	filesA := make(map[string]FileEntry)
	filesB := make(map[string]FileEntry)
	for _, f := range a.Files {
		filesA[f.Path] = f
	}
	for _, f := range b.Files {
		filesB[f.Path] = f
	}

	// Added files
	var added []string
	for path := range filesB {
		if _, exists := filesA[path]; !exists {
			added = append(added, path)
		}
	}
	sort.Strings(added)

	// Deleted files
	var deleted []string
	for path := range filesA {
		if _, exists := filesB[path]; !exists {
			deleted = append(deleted, path)
		}
	}
	sort.Strings(deleted)

	// Modified files
	var modified []string
	for path, fa := range filesA {
		if fb, exists := filesB[path]; exists {
			if fa.Checksum != fb.Checksum {
				modified = append(modified, path)
			}
		}
	}
	sort.Strings(modified)

	// Unchanged
	unchanged := len(filesA) + len(added) - len(deleted) - len(modified)

	if len(added) > 0 {
		sb.WriteString(fmt.Sprintf("  + Added (%d):\n", len(added)))
		for _, p := range added {
			sb.WriteString(fmt.Sprintf("    + %s\n", p))
		}
	}

	if len(deleted) > 0 {
		sb.WriteString(fmt.Sprintf("  - Deleted (%d):\n", len(deleted)))
		for _, p := range deleted {
			sb.WriteString(fmt.Sprintf("    - %s\n", p))
		}
	}

	if len(modified) > 0 {
		sb.WriteString(fmt.Sprintf("  ~ Modified (%d):\n", len(modified)))
		for _, p := range modified {
			fa := filesA[p]
			fb := filesB[p]
			sb.WriteString(fmt.Sprintf("    ~ %s  (%d → %d bytes)\n", p, fa.Size, fb.Size))
		}
	}

	sb.WriteString(fmt.Sprintf("\n  = Unchanged: %d files\n", unchanged))

	// Git state diff
	if cpA.GitBranch != cpB.GitBranch {
		sb.WriteString(fmt.Sprintf("\n  Branch: %s → %s\n", cpA.GitBranch, cpB.GitBranch))
	}
	if cpA.GitCommit != cpB.GitCommit {
		sb.WriteString(fmt.Sprintf("  Commit: %s → %s\n", cpA.GitCommit, cpB.GitCommit))
	}

	return sb.String()
}

// NameOrID returns the name if set, otherwise the ID.
func (c *Checkpoint) NameOrID() string {
	if c.Name != "" {
		return c.Name
	}
	return c.ID
}
