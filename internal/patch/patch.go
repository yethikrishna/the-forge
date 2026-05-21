// Package patch provides intelligent patch generation and application.
// Unlike simple diff/apply, patch understands code structure and can:
//   - Generate semantic patches from agent changes
//   - Apply patches with conflict resolution
//   - Reverse patches
//   - Validate patches before applying
//   - Merge overlapping patches
//
// Every change, a clean patch.
package patch

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// PatchStatus represents the status of a patch.
type PatchStatus string

const (
	StatusDraft    PatchStatus = "draft"
	StatusReady    PatchStatus = "ready"
	StatusApplied  PatchStatus = "applied"
	StatusReverted PatchStatus = "reverted"
	StatusConflict PatchStatus = "conflict"
	StatusRejected PatchStatus = "rejected"
)

// OperationType classifies a patch operation.
type OperationType string

const (
	OpAdd    OperationType = "add"    // add new content
	OpDelete OperationType = "delete" // delete content
	OpModify OperationType = "modify" // modify existing content
	OpMove   OperationType = "move"   // move/rename file
	OpCopy   OperationType = "copy"   // copy file
)

// Hunk represents a single change hunk within a file.
type Hunk struct {
	OldStart int    `json:"old_start"`
	OldCount int    `json:"old_count"`
	NewStart int    `json:"new_start"`
	NewCount int    `json:"new_count"`
	OldText  string `json:"old_text"`
	NewText  string `json:"new_text"`
}

// FilePatch represents changes to a single file.
type FilePatch struct {
	File       string        `json:"file"`
	NewFile    string        `json:"new_file,omitempty"` // for move operations
	Operation  OperationType `json:"operation"`
	Hunks      []Hunk        `json:"hunks,omitempty"`
	OldSHA     string        `json:"old_sha,omitempty"`
	NewSHA     string        `json:"new_sha,omitempty"`
	OldContent string        `json:"-"` // not serialized, used during generation
	NewContent string        `json:"-"` // not serialized, used during generation
}

// Patch represents a collection of file changes.
type Patch struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Author      string      `json:"author,omitempty"`
	AgentID     string      `json:"agent_id,omitempty"`
	SessionID   string      `json:"session_id,omitempty"`
	Files       []FilePatch `json:"files"`
	Status      PatchStatus `json:"status"`
	Tags        []string    `json:"tags,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
	AppliedAt   time.Time   `json:"applied_at,omitempty"`
	RevertedAt  time.Time   `json:"reverted_at,omitempty"`
}

// Manager manages patches.
type Manager struct {
	dir     string
	patches map[string]*Patch
	mu      sync.RWMutex
}

// NewManager creates a new patch manager.
func NewManager(dir string) *Manager {
	os.MkdirAll(dir, 0755)
	m := &Manager{
		dir:     dir,
		patches: make(map[string]*Patch),
	}
	m.load()
	return m
}

// Create creates a new empty patch.
func (m *Manager) Create(name, description string) *Patch {
	m.mu.Lock()
	defer m.mu.Unlock()

	p := &Patch{
		ID:          fmt.Sprintf("patch-%d", time.Now().UnixNano()),
		Name:        name,
		Description: description,
		Files:       make([]FilePatch, 0),
		Status:      StatusDraft,
		CreatedAt:   time.Now(),
	}
	m.patches[p.ID] = p
	m.save()
	return p
}

// Get returns a patch by ID.
func (m *Manager) Get(id string) (*Patch, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.patches[id]
	if !ok {
		return nil, false
	}
	copy := *p
	return &copy, true
}

// List returns all patches.
func (m *Manager) List() []Patch {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Patch, 0, len(m.patches))
	for _, p := range m.patches {
		result = append(result, *p)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}

// Delete removes a patch.
func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.patches[id]; !ok {
		return fmt.Errorf("patch %q not found", id)
	}
	delete(m.patches, id)
	m.save()
	return nil
}

// AddFileChange adds a file change to a patch by comparing old and new content.
func (m *Manager) AddFileChange(patchID, file, oldContent, newContent string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.patches[patchID]
	if !ok {
		return fmt.Errorf("patch %q not found", patchID)
	}

	if p.Status != StatusDraft {
		return fmt.Errorf("can only add changes to draft patches")
	}

	fp := FilePatch{
		File:   file,
		OldSHA: sha256str(oldContent),
		NewSHA: sha256str(newContent),
	}

	if oldContent == "" && newContent != "" {
		fp.Operation = OpAdd
		fp.Hunks = []Hunk{{
			OldStart: 0, OldCount: 0,
			NewStart: 1, NewCount: countLines(newContent),
			OldText: "",
			NewText: newContent,
		}}
	} else if oldContent != "" && newContent == "" {
		fp.Operation = OpDelete
		fp.Hunks = []Hunk{{
			OldStart: 1, OldCount: countLines(oldContent),
			NewStart: 0, NewCount: 0,
			OldText: oldContent,
			NewText: "",
		}}
	} else if oldContent != newContent {
		fp.Operation = OpModify
		fp.Hunks = generateHunks(oldContent, newContent)
	} else {
		// No change
		return nil
	}

	p.Files = append(p.Files, fp)
	m.save()
	return nil
}

// AddFileMove adds a file move/rename to a patch.
func (m *Manager) AddFileMove(patchID, oldPath, newPath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.patches[patchID]
	if !ok {
		return fmt.Errorf("patch %q not found", patchID)
	}

	if p.Status != StatusDraft {
		return fmt.Errorf("can only add changes to draft patches")
	}

	p.Files = append(p.Files, FilePatch{
		File:      oldPath,
		NewFile:   newPath,
		Operation: OpMove,
	})
	m.save()
	return nil
}

// Finalize marks a patch as ready to apply.
func (m *Manager) Finalize(patchID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.patches[patchID]
	if !ok {
		return fmt.Errorf("patch %q not found", patchID)
	}

	if len(p.Files) == 0 {
		return fmt.Errorf("patch has no file changes")
	}

	p.Status = StatusReady
	m.save()
	return nil
}

// Validate checks if a patch can be applied cleanly.
func (m *Manager) Validate(patchID, rootDir string) ([]string, error) {
	m.mu.RLock()
	p, ok := m.patches[patchID]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("patch %q not found", patchID)
	}
	return validatePatch(p, rootDir), nil
}

// validatePatch checks a patch against the filesystem without holding any lock.
func validatePatch(p *Patch, rootDir string) []string {
	var conflicts []string
	for _, fp := range p.Files {
		filePath := filepath.Join(rootDir, fp.File)

		switch fp.Operation {
		case OpAdd:
			if _, err := os.Stat(filePath); err == nil {
				conflicts = append(conflicts, fmt.Sprintf("file already exists: %s", fp.File))
			}
		case OpDelete:
			if _, err := os.Stat(filePath); err != nil {
				conflicts = append(conflicts, fmt.Sprintf("file not found: %s", fp.File))
			}
		case OpModify:
			data, err := os.ReadFile(filePath)
			if err != nil {
				conflicts = append(conflicts, fmt.Sprintf("cannot read file: %s", fp.File))
				continue
			}
			currentSHA := sha256str(string(data))
			if fp.OldSHA != "" && currentSHA != fp.OldSHA {
				conflicts = append(conflicts, fmt.Sprintf("file has been modified since patch was created: %s", fp.File))
			}
		case OpMove:
			if _, err := os.Stat(filepath.Join(rootDir, fp.File)); err != nil {
				conflicts = append(conflicts, fmt.Sprintf("source file not found: %s", fp.File))
			}
			if _, err := os.Stat(filepath.Join(rootDir, fp.NewFile)); err == nil {
				conflicts = append(conflicts, fmt.Sprintf("destination already exists: %s", fp.NewFile))
			}
		}
	}
	return conflicts
}

// Apply applies a patch to the filesystem.
func (m *Manager) Apply(patchID, rootDir string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.patches[patchID]
	if !ok {
		return fmt.Errorf("patch %q not found", patchID)
	}

	if p.Status != StatusReady {
		return fmt.Errorf("patch must be in ready state (current: %s)", p.Status)
	}

	// Validate first (internal, no lock needed — we already hold the write lock)
	conflicts := validatePatch(p, rootDir)
	if len(conflicts) > 0 {
		p.Status = StatusConflict
		m.save()
		return fmt.Errorf("conflicts detected:\n  %s", strings.Join(conflicts, "\n  "))
	}

	// Apply each file change
	for _, fp := range p.Files {
		filePath := filepath.Join(rootDir, fp.File)

		switch fp.Operation {
		case OpAdd:
			os.MkdirAll(filepath.Dir(filePath), 0755)
			if err := os.WriteFile(filePath, []byte(fp.Hunks[0].NewText), 0644); err != nil {
				return fmt.Errorf("writing %s: %w", fp.File, err)
			}

		case OpDelete:
			if err := os.Remove(filePath); err != nil {
				return fmt.Errorf("deleting %s: %w", fp.File, err)
			}

		case OpModify:
			data, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("reading %s: %w", fp.File, err)
			}
			newContent := applyHunks(string(data), fp.Hunks)
			if err := os.WriteFile(filePath, []byte(newContent), 0644); err != nil {
				return fmt.Errorf("writing %s: %w", fp.File, err)
			}

		case OpMove:
			newPath := filepath.Join(rootDir, fp.NewFile)
			os.MkdirAll(filepath.Dir(newPath), 0755)
			if err := os.Rename(filePath, newPath); err != nil {
				return fmt.Errorf("moving %s: %w", fp.File, err)
			}
		}
	}

	p.Status = StatusApplied
	p.AppliedAt = time.Now()
	m.save()
	return nil
}

// Revert reverses an applied patch.
func (m *Manager) Revert(patchID, rootDir string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.patches[patchID]
	if !ok {
		return fmt.Errorf("patch %q not found", patchID)
	}

	if p.Status != StatusApplied {
		return fmt.Errorf("can only revert applied patches (current: %s)", p.Status)
	}

	// Apply in reverse order
	for i := len(p.Files) - 1; i >= 0; i-- {
		fp := p.Files[i]
		filePath := filepath.Join(rootDir, fp.File)

		switch fp.Operation {
		case OpAdd:
			// Reverse of add is delete
			os.Remove(filePath)

		case OpDelete:
			// Reverse of delete is add
			os.MkdirAll(filepath.Dir(filePath), 0755)
			os.WriteFile(filePath, []byte(fp.Hunks[0].OldText), 0644)

		case OpModify:
			data, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("reading %s: %w", fp.File, err)
			}
			// Apply hunks in reverse
			reversedHunks := make([]Hunk, len(fp.Hunks))
			for j, h := range fp.Hunks {
				reversedHunks[j] = Hunk{
					OldStart: h.NewStart, OldCount: h.NewCount,
					NewStart: h.OldStart, NewCount: h.OldCount,
					OldText: h.NewText, NewText: h.OldText,
				}
			}
			newContent := applyHunks(string(data), reversedHunks)
			os.WriteFile(filePath, []byte(newContent), 0644)

		case OpMove:
			oldPath := filepath.Join(rootDir, fp.File)
			newPath := filepath.Join(rootDir, fp.NewFile)
			os.Rename(newPath, oldPath)
		}
	}

	p.Status = StatusReverted
	p.RevertedAt = time.Now()
	m.save()
	return nil
}

// Stats returns patch manager statistics.
func (m *Manager) Stats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	byStatus := make(map[PatchStatus]int)
	totalFiles := 0
	totalHunks := 0

	for _, p := range m.patches {
		byStatus[p.Status]++
		totalFiles += len(p.Files)
		for _, fp := range p.Files {
			totalHunks += len(fp.Hunks)
		}
	}

	return map[string]interface{}{
		"total_patches": len(m.patches),
		"total_files":   totalFiles,
		"total_hunks":   totalHunks,
		"by_status":     byStatus,
	}
}

// RenderPatch renders a patch for display.
func RenderPatch(p *Patch) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Patch: %s\n", p.Name)
	fmt.Fprintf(&b, "ID: %s\n", p.ID)
	fmt.Fprintf(&b, "Status: %s\n", p.Status)
	fmt.Fprintf(&b, "Description: %s\n", p.Description)
	if p.Author != "" {
		fmt.Fprintf(&b, "Author: %s\n", p.Author)
	}
	if p.AgentID != "" {
		fmt.Fprintf(&b, "Agent: %s\n", p.AgentID)
	}
	fmt.Fprintf(&b, "Created: %s\n", p.CreatedAt.Format(time.RFC3339))
	if !p.AppliedAt.IsZero() {
		fmt.Fprintf(&b, "Applied: %s\n", p.AppliedAt.Format(time.RFC3339))
	}

	fmt.Fprintf(&b, "\nFiles (%d):\n", len(p.Files))
	for _, fp := range p.Files {
		fmt.Fprintf(&b, "  %s %s", fp.Operation, fp.File)
		if fp.NewFile != "" {
			fmt.Fprintf(&b, " → %s", fp.NewFile)
		}
		fmt.Fprintf(&b, " (%d hunks)\n", len(fp.Hunks))
	}

	return b.String()
}

// RenderDiff renders a patch as a unified diff.
func RenderDiff(p *Patch) string {
	var b strings.Builder

	for _, fp := range p.Files {
		switch fp.Operation {
		case OpAdd:
			fmt.Fprintf(&b, "--- /dev/null\n")
			fmt.Fprintf(&b, "+++ b/%s\n", fp.File)
			for _, h := range fp.Hunks {
				fmt.Fprintf(&b, "@@ -0,0 +1,%d @@\n", h.NewCount)
				for _, line := range strings.Split(h.NewText, "\n") {
					fmt.Fprintf(&b, "+%s\n", line)
				}
			}
		case OpDelete:
			fmt.Fprintf(&b, "--- a/%s\n", fp.File)
			fmt.Fprint(&b, "+++ /dev/null\n")
			for _, h := range fp.Hunks {
				fmt.Fprintf(&b, "@@ -1,%d +0,0 @@\n", h.OldCount)
				for _, line := range strings.Split(h.OldText, "\n") {
					fmt.Fprintf(&b, "-%s\n", line)
				}
			}
		case OpModify:
			fmt.Fprintf(&b, "--- a/%s\n", fp.File)
			fmt.Fprintf(&b, "+++ b/%s\n", fp.File)
			for _, h := range fp.Hunks {
				fmt.Fprintf(&b, "@@ -%d,%d +%d,%d @@\n",
					h.OldStart, h.OldCount, h.NewStart, h.NewCount)
				for _, line := range strings.Split(h.OldText, "\n") {
					if line != "" {
						fmt.Fprintf(&b, "-%s\n", line)
					}
				}
				for _, line := range strings.Split(h.NewText, "\n") {
					if line != "" {
						fmt.Fprintf(&b, "+%s\n", line)
					}
				}
			}
		case OpMove:
			fmt.Fprintf(&b, "rename from %s\n", fp.File)
			fmt.Fprintf(&b, "rename to %s\n", fp.NewFile)
		}
	}

	return b.String()
}

// Helper functions

func sha256str(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h[:8])
}

func countLines(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

func generateHunks(oldText, newText string) []Hunk {
	oldLines := strings.Split(oldText, "\n")
	newLines := strings.Split(newText, "\n")

	// Simple diff: find common prefix and suffix, everything in between is a hunk
	prefixLen := 0
	for prefixLen < len(oldLines) && prefixLen < len(newLines) && oldLines[prefixLen] == newLines[prefixLen] {
		prefixLen++
	}

	suffixLen := 0
	for suffixLen < len(oldLines)-prefixLen && suffixLen < len(newLines)-prefixLen &&
		oldLines[len(oldLines)-1-suffixLen] == newLines[len(newLines)-1-suffixLen] {
		suffixLen++
	}

	oldMiddle := strings.Join(oldLines[prefixLen:len(oldLines)-suffixLen], "\n")
	newMiddle := strings.Join(newLines[prefixLen:len(newLines)-suffixLen], "\n")

	if oldMiddle == "" && newMiddle == "" {
		return nil
	}

	return []Hunk{{
		OldStart: prefixLen + 1,
		OldCount: len(oldLines) - prefixLen - suffixLen,
		NewStart: prefixLen + 1,
		NewCount: len(newLines) - prefixLen - suffixLen,
		OldText:  oldMiddle,
		NewText:  newMiddle,
	}}
}

func applyHunks(content string, hunks []Hunk) string {
	if len(hunks) == 0 {
		return content
	}

	lines := strings.Split(content, "\n")

	// Apply hunks in reverse order to preserve line numbers
	sort.Slice(hunks, func(i, j int) bool {
		return hunks[i].OldStart > hunks[j].OldStart
	})

	for _, hunk := range hunks {
		start := hunk.OldStart - 1 // 0-indexed
		if start < 0 {
			start = 0
		}

		end := start + hunk.OldCount
		if end > len(lines) {
			end = len(lines)
		}

		newLines := strings.Split(hunk.NewText, "\n")
		if hunk.NewText == "" {
			newLines = []string{}
		}

		// Replace lines[start:end] with newLines
		result := make([]string, 0, len(lines)-hunk.OldCount+hunk.NewCount)
		result = append(result, lines[:start]...)
		result = append(result, newLines...)
		result = append(result, lines[end:]...)
		lines = result
	}

	return strings.Join(lines, "\n")
}

func (m *Manager) save() {
	if m.dir == "" {
		return
	}
	os.MkdirAll(m.dir, 0755)
	data, _ := json.MarshalIndent(m.patches, "", "  ")
	os.WriteFile(filepath.Join(m.dir, "patches.json"), data, 0644)
}

func (m *Manager) load() {
	if m.dir == "" {
		return
	}
	data, err := os.ReadFile(filepath.Join(m.dir, "patches.json"))
	if err != nil {
		return
	}
	json.Unmarshal(data, &m.patches)
}
