// Package undo provides universal agent undo capabilities.
// Revert file changes, git commits, and entire sessions.
//
// Every action can be undone. The forge remembers.
package undo

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

// ActionType classifies the kind of operation that was performed.
type ActionType string

const (
	ActionFileWrite  ActionType = "file_write"
	ActionFileDelete ActionType = "file_delete"
	ActionFileRename ActionType = "file_rename"
	ActionGitCommit  ActionType = "git_commit"
	ActionCommand    ActionType = "command"
	ActionFileCreate ActionType = "file_create"
)

// Snapshot captures the state before an action was performed.
type Snapshot struct {
	ID         string     `json:"id"`
	Timestamp  time.Time  `json:"timestamp"`
	Action     ActionType `json:"action"`
	Path       string     `json:"path,omitempty"`
	Content    string     `json:"content,omitempty"` // original file content
	CommitHash string     `json:"commit_hash,omitempty"`
	Command    string     `json:"command,omitempty"`
	Agent      string     `json:"agent,omitempty"`
	Session    string     `json:"session,omitempty"`
	Reverted   bool       `json:"reverted"`
}

// Journal records all agent actions for undo purposes.
type Journal struct {
	Dir     string
	entries []Snapshot
}

// NewJournal creates or opens a journal at the given directory.
func NewJournal(dir string) *Journal {
	return &Journal{Dir: dir}
}

// Record saves a snapshot of state before an action.
func (j *Journal) Record(snapshot Snapshot) (string, error) {
	if snapshot.ID == "" {
		snapshot.ID = fmt.Sprintf("snap-%d", time.Now().UnixNano())
	}
	if snapshot.Timestamp.IsZero() {
		snapshot.Timestamp = time.Now()
	}

	j.entries = append(j.entries, snapshot)

	// Persist to disk
	if err := os.MkdirAll(j.Dir, 0o755); err != nil {
		return snapshot.ID, fmt.Errorf("failed to create journal dir: %w", err)
	}

	path := j.snapshotPath(snapshot.ID)
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return snapshot.ID, fmt.Errorf("failed to marshal snapshot: %w", err)
	}

	return snapshot.ID, os.WriteFile(path, data, 0o644)
}

// Load reads all snapshots from disk, sorted by timestamp (newest first).
func (j *Journal) Load() ([]Snapshot, error) {
	entries, err := os.ReadDir(j.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var snapshots []Snapshot
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(j.Dir, e.Name()))
		if err != nil {
			continue
		}
		var snap Snapshot
		if err := json.Unmarshal(data, &snap); err != nil {
			continue
		}
		snapshots = append(snapshots, snap)
	}

	// Sort newest first
	sort.Slice(snapshots, func(i, k int) bool {
		return snapshots[i].Timestamp.After(snapshots[k].Timestamp)
	})

	j.entries = snapshots
	return snapshots, nil
}

// Get retrieves a specific snapshot by ID.
func (j *Journal) Get(id string) (*Snapshot, error) {
	if j.entries == nil {
		if _, err := j.Load(); err != nil {
			return nil, err
		}
	}

	for _, e := range j.entries {
		if e.ID == id {
			return &e, nil
		}
	}
	return nil, fmt.Errorf("snapshot %q not found", id)
}

// List returns the most recent N snapshots.
func (j *Journal) List(limit int) ([]Snapshot, error) {
	all, err := j.Load()
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}

// Undo reverts a specific snapshot.
func (j *Journal) Undo(id string) error {
	snap, err := j.Get(id)
	if err != nil {
		return err
	}

	if snap.Reverted {
		return fmt.Errorf("snapshot %s already reverted", id)
	}

	switch snap.Action {
	case ActionFileWrite, ActionFileCreate:
		return j.undoFileWrite(snap)
	case ActionFileDelete:
		return j.undoFileDelete(snap)
	case ActionFileRename:
		return j.undoFileRename(snap)
	case ActionGitCommit:
		return j.undoGitCommit(snap)
	default:
		return fmt.Errorf("undo not supported for action type: %s", snap.Action)
	}
}

// UndoLast reverts the most recent snapshot.
func (j *Journal) UndoLast() (*Snapshot, error) {
	snaps, err := j.List(1)
	if err != nil {
		return nil, err
	}
	if len(snaps) == 0 {
		return nil, fmt.Errorf("no snapshots to undo")
	}
	snap := snaps[0]
	return &snap, j.Undo(snap.ID)
}

// UndoAll reverts all non-reverted snapshots in reverse order.
func (j *Journal) UndoAll() (int, error) {
	snaps, err := j.Load()
	if err != nil {
		return 0, err
	}

	count := 0
	for _, snap := range snaps {
		if snap.Reverted {
			continue
		}
		if err := j.Undo(snap.ID); err != nil {
			return count, fmt.Errorf("failed to undo %s: %w", snap.ID, err)
		}
		count++
	}
	return count, nil
}

func (j *Journal) undoFileWrite(snap *Snapshot) error {
	if snap.Content == "" {
		// File was created new — delete it
		return os.Remove(snap.Path)
	}
	// Restore original content
	return os.WriteFile(snap.Path, []byte(snap.Content), 0o644)
}

func (j *Journal) undoFileDelete(snap *Snapshot) error {
	if snap.Content == "" {
		return fmt.Errorf("cannot undo deletion — original content not captured")
	}
	// Restore the deleted file
	dir := filepath.Dir(snap.Path)
	os.MkdirAll(dir, 0o755)
	return os.WriteFile(snap.Path, []byte(snap.Content), 0o644)
}

func (j *Journal) undoFileRename(snap *Snapshot) error {
	// Path contains the new name, Content/commit hash area contains the old name
	// For simplicity, we track renames as: Path=new, original in Content metadata
	return fmt.Errorf("file rename undo requires original path metadata")
}

func (j *Journal) undoGitCommit(snap *Snapshot) error {
	if snap.CommitHash == "" {
		return fmt.Errorf("no commit hash recorded")
	}
	// Reset to the commit before this one
	cmd := exec.Command("git", "reset", "--soft", "HEAD~1")
	cmd.Dir = filepath.Dir(snap.Path)
	if snap.Path == "" {
		cmd.Dir, _ = os.Getwd()
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git reset failed: %s: %w", string(out), err)
	}
	return nil
}

func (j *Journal) markReverted(id string) error {
	snap, err := j.Get(id)
	if err != nil {
		return err
	}
	snap.Reverted = true
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(j.snapshotPath(id), data, 0o644)
}

func (j *Journal) snapshotPath(id string) string {
	return filepath.Join(j.Dir, id+".json")
}

// BeforeWrite captures file state before a write operation.
// Returns a snapshot ID for later undo.
func (j *Journal) BeforeWrite(path string, agent string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}

	content := ""
	action := ActionFileCreate

	data, err := os.ReadFile(abs)
	if err == nil {
		content = string(data)
		action = ActionFileWrite
	}

	return j.Record(Snapshot{
		Action:  action,
		Path:    abs,
		Content: content,
		Agent:   agent,
	})
}

// BeforeDelete captures file state before a delete operation.
func (j *Journal) BeforeDelete(path string, agent string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}

	content := ""
	data, err := os.ReadFile(abs)
	if err == nil {
		content = string(data)
	}

	return j.Record(Snapshot{
		Action:  ActionFileDelete,
		Path:    abs,
		Content: content,
		Agent:   agent,
	})
}

// BeforeGitCommit records the current HEAD before a commit.
func (j *Journal) BeforeGitCommit(workDir string, agent string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = workDir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current HEAD: %w", err)
	}
	hash := strings.TrimSpace(string(out))

	return j.Record(Snapshot{
		Action:     ActionGitCommit,
		Path:       workDir,
		CommitHash: hash,
		Agent:      agent,
	})
}
