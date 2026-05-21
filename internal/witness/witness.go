// Package witness provides cryptographic proof of agent actions using Merkle trees.
// Every action an agent takes is recorded as a leaf in a Merkle tree,
// producing a tamper-evident audit trail with efficient verification.
//
// Prove it happened.
package witness

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Action represents an agent action that can be witnessed.
type Action struct {
	ID        string                 `json:"id"`
	AgentID   string                 `json:"agent_id"`
	Type      string                 `json:"type"` // "file_write", "file_read", "command", "api_call", "message"
	Target    string                 `json:"target"`
	Detail    string                 `json:"detail,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	SessionID string                 `json:"session_id"`
}

// Proof represents a Merkle proof for a specific action.
type Proof struct {
	ActionID    string   `json:"action_id"`
	LeafHash    string   `json:"leaf_hash"`
	RootHash    string   `json:"root_hash"`
	ProofHashes []string `json:"proof_hashes"`
	ProofDir    []bool   `json:"proof_dir"` // true = right, false = left
	TreeSize    int      `json:"tree_size"`
	Verified    bool     `json:"verified"`
}

// Tree is a Merkle tree of witnessed actions.
type Tree struct {
	mu      sync.RWMutex
	Leaves  []string `json:"leaves"`
	Actions []Action `json:"actions"`
	Root    string   `json:"root"`
	dirty   bool
}

// Witness records and proves agent actions.
type Witness struct {
	mu    sync.Mutex
	dir   string
	trees map[string]*Tree // session_id → tree
}

// NewWitness creates a new witness.
func NewWitness(dir string) (*Witness, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	w := &Witness{
		dir:   dir,
		trees: make(map[string]*Tree),
	}
	w.load()
	return w, nil
}

// Record records an agent action in the Merkle tree.
func (w *Witness) Record(action Action) (string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if action.ID == "" {
		action.ID = fmt.Sprintf("act-%d", time.Now().UnixNano())
	}
	if action.Timestamp.IsZero() {
		action.Timestamp = time.Now()
	}

	// Get or create tree for session
	tree, ok := w.trees[action.SessionID]
	if !ok {
		tree = &Tree{}
		w.trees[action.SessionID] = tree
	}

	// Hash the action
	data, err := json.Marshal(action)
	if err != nil {
		return "", fmt.Errorf("witness: marshal action: %w", err)
	}
	leafHash := sha256.Sum256(data)
	leafHex := hex.EncodeToString(leafHash[:])

	tree.Leaves = append(tree.Leaves, leafHex)
	tree.Actions = append(tree.Actions, action)
	tree.dirty = true

	// Save incrementally
	w.saveTree(action.SessionID)

	return action.ID, nil
}

// RootHash returns the current Merkle root for a session.
func (w *Witness) RootHash(sessionID string) (string, error) {
	w.mu.Lock()
	tree, ok := w.trees[sessionID]
	w.mu.Unlock()

	if !ok {
		return "", fmt.Errorf("witness: unknown session: %s", sessionID)
	}

	tree.mu.Lock()
	defer tree.mu.Unlock()

	if tree.dirty {
		tree.Root = computeRoot(tree.Leaves)
		tree.dirty = false
	}

	return tree.Root, nil
}

// Prove generates a Merkle proof for a specific action.
func (w *Witness) Prove(sessionID, actionID string) (*Proof, error) {
	w.mu.Lock()
	tree, ok := w.trees[sessionID]
	w.mu.Unlock()

	if !ok {
		return nil, fmt.Errorf("witness: unknown session: %s", sessionID)
	}

	tree.mu.RLock()
	defer tree.mu.RUnlock()

	// Find the action
	idx := -1
	for i, a := range tree.Actions {
		if a.ID == actionID {
			idx = i
			break
		}
	}

	if idx < 0 {
		return nil, fmt.Errorf("witness: action not found: %s", actionID)
	}

	leafHash := tree.Leaves[idx]
	rootHash := computeRoot(tree.Leaves)
	proofHashes, proofDir := merkleProof(tree.Leaves, idx)

	return &Proof{
		ActionID:    actionID,
		LeafHash:    leafHash,
		RootHash:    rootHash,
		ProofHashes: proofHashes,
		ProofDir:    proofDir,
		TreeSize:    len(tree.Leaves),
	}, nil
}

// Verify checks a Merkle proof against the stored root.
func (w *Witness) Verify(proof *Proof) bool {
	currentHash := proof.LeafHash

	for i, hash := range proof.ProofHashes {
		combined := hash + currentHash
		if proof.ProofDir[i] {
			combined = currentHash + hash
		}
		h := sha256.Sum256([]byte(combined))
		currentHash = hex.EncodeToString(h[:])
	}

	return currentHash == proof.RootHash
}

// VerifyStandalone verifies a proof without needing the witness.
func VerifyStandalone(proof *Proof) bool {
	currentHash := proof.LeafHash

	for i, hash := range proof.ProofHashes {
		combined := hash + currentHash
		if proof.ProofDir[i] {
			combined = currentHash + hash
		}
		h := sha256.Sum256([]byte(combined))
		currentHash = hex.EncodeToString(h[:])
	}

	return currentHash == proof.RootHash
}

// ListSessions returns all session IDs with witnessed actions.
func (w *Witness) ListSessions() []string {
	w.mu.Lock()
	defer w.mu.Unlock()

	sessions := make([]string, 0, len(w.trees))
	for id := range w.trees {
		sessions = append(sessions, id)
	}
	return sessions
}

// GetActions returns all actions for a session.
func (w *Witness) GetActions(sessionID string) []Action {
	w.mu.Lock()
	tree, ok := w.trees[sessionID]
	w.mu.Unlock()

	if !ok {
		return nil
	}

	tree.mu.RLock()
	defer tree.mu.RUnlock()

	actions := make([]Action, len(tree.Actions))
	copy(actions, tree.Actions)
	return actions
}

// computeRoot computes the Merkle root from leaf hashes.
func computeRoot(leaves []string) string {
	if len(leaves) == 0 {
		return hex.EncodeToString(make([]byte, 32))
	}
	if len(leaves) == 1 {
		return leaves[0]
	}

	var level []string
	for _, l := range leaves {
		level = append(level, l)
	}

	for len(level) > 1 {
		var next []string
		for i := 0; i < len(level); i += 2 {
			if i+1 < len(level) {
				h := sha256.Sum256([]byte(level[i] + level[i+1]))
				next = append(next, hex.EncodeToString(h[:]))
			} else {
				h := sha256.Sum256([]byte(level[i] + level[i]))
				next = append(next, hex.EncodeToString(h[:]))
			}
		}
		level = next
	}

	return level[0]
}

// merkleProof generates the sibling hashes needed to verify a leaf.
func merkleProof(leaves []string, idx int) ([]string, []bool) {
	var proofHashes []string
	var proofDir []bool

	level := make([]string, len(leaves))
	copy(level, leaves)

	currentIdx := idx

	for len(level) > 1 {
		var next []string
		siblingIdx := currentIdx ^ 1 // XOR to get sibling

		if siblingIdx < len(level) {
			proofHashes = append(proofHashes, level[siblingIdx])
			proofDir = append(proofDir, currentIdx%2 == 0) // sibling is on right if current is even
		} else {
			proofHashes = append(proofHashes, level[currentIdx])
			proofDir = append(proofDir, false)
		}

		for i := 0; i < len(level); i += 2 {
			if i+1 < len(level) {
				h := sha256.Sum256([]byte(level[i] + level[i+1]))
				next = append(next, hex.EncodeToString(h[:]))
			} else {
				h := sha256.Sum256([]byte(level[i] + level[i]))
				next = append(next, hex.EncodeToString(h[:]))
			}
		}

		currentIdx /= 2
		level = next
	}

	return proofHashes, proofDir
}

// load reads all trees from disk.
func (w *Witness) load() {
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(w.dir, e.Name()))
		if err != nil {
			continue
		}

		var tree Tree
		if err := json.Unmarshal(data, &tree); err != nil {
			continue
		}

		sessionID := strings.TrimSuffix(e.Name(), ".json")
		tree.dirty = true
		w.trees[sessionID] = &tree
	}
}

// saveTree persists a tree to disk.
func (w *Witness) saveTree(sessionID string) {
	tree, ok := w.trees[sessionID]
	if !ok {
		return
	}

	data, err := json.MarshalIndent(tree, "", "  ")
	if err != nil {
		return
	}

	os.WriteFile(filepath.Join(w.dir, sessionID+".json"), data, 0o644)
}

// FormatAction renders an action for display.
func FormatAction(a Action) string {
	return fmt.Sprintf("%s  %-12s  %-15s  %s  %s",
		a.Timestamp.Format("15:04:05"), a.Type, a.AgentID, a.Target, a.Detail)
}

// FormatProof renders a proof for display.
func FormatProof(p *Proof) string {
	verified := "❌"
	if p.Verified {
		verified = "✓"
	}
	return fmt.Sprintf("%s Action:%s  Root:%s…%s  Path:%d steps",
		verified, p.ActionID, p.RootHash[:8], p.RootHash[len(p.RootHash)-4:], len(p.ProofHashes))
}
