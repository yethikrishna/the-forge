// Package crossdevice provides cross-device persistent context.
// Same agent brain, any device, zero context loss via differential sync.
package crossdevice

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"
)

// DeviceType categorizes a device.
type DeviceType int

const (
	DeviceLaptop DeviceType = iota
	DevicePhone
	DeviceTablet
	DeviceDesktop
	DeviceServer
)

func (d DeviceType) String() string {
	return [...]string{"laptop", "phone", "tablet", "desktop", "server"}[d]
}

// DeviceProfile describes a registered device.
type DeviceProfile struct {
	ID             string
	Type           DeviceType
	Name           string
	Capabilities   []string // "browser", "notifications", "voice", etc.
	LastSync       time.Time
	IsActive       bool
	TrustLevel     float64 // 0-1
}

// BlobType categorizes a context blob.
type BlobType int

const (
	BlobConversation BlobType = iota
	BlobMemory
	BlobWorkspace
	BlobAuth
	BlobPreference
)

func (b BlobType) String() string {
	return [...]string{"conversation", "memory", "workspace", "auth", "preference"}[b]
}

// ContextBlob is a typed, versioned piece of agent state.
type ContextBlob struct {
	ID        string
	Type      BlobType
	Version   int
	Data      []byte
	Hash      string // SHA-256 of data
	DeviceID  string // last modified by
	Modified  time.Time
}

// ComputeHash calculates the SHA-256 hash of blob data.
func (b *ContextBlob) ComputeHash() string {
	h := sha256.Sum256(b.Data)
	return fmt.Sprintf("%x", h[:])
}

// SyncResult captures the outcome of a sync operation.
type SyncResult struct {
	DeviceID    string
	Uploaded    int
	Downloaded  int
	Conflicts   []SyncConflict
	Duration    time.Duration
}

// SyncConflict occurs when the same blob is modified on two devices.
type SyncConflict struct {
	BlobID      string
	LocalVer    int
	RemoteVer   int
	Resolution  ResolutionStrategy
}

// ResolutionStrategy defines how to resolve conflicts.
type ResolutionStrategy int

const (
	ResolveAutoMerge ResolutionStrategy = iota
	ResolveLastWriterWins
	ResolveManual
	ResolveFork
)

// CrossDevice is the main sync engine.
type CrossDevice struct {
	devices map[string]*DeviceProfile
	blobs   map[string]*ContextBlob // blobID → current version
	history map[string][]ContextBlob // blobID → version history
	active  string                  // currently active device ID
	mu      sync.RWMutex
}

// New creates a new cross-device sync engine.
func New() *CrossDevice {
	return &CrossDevice{
		devices: make(map[string]*DeviceProfile),
		blobs:   make(map[string]*ContextBlob),
		history: make(map[string][]ContextBlob),
	}
}

// RegisterDevice adds a device to the sync pool.
func (cd *CrossDevice) RegisterDevice(device DeviceProfile) error {
	cd.mu.Lock()
	defer cd.mu.Unlock()
	if _, exists := cd.devices[device.ID]; exists {
		return fmt.Errorf("device %s already registered", device.ID)
	}
	cd.devices[device.ID] = &device
	return nil
}

// SetActive marks a device as the primary device.
func (cd *CrossDevice) SetActive(deviceID string) error {
	cd.mu.Lock()
	defer cd.mu.Unlock()
	if _, ok := cd.devices[deviceID]; !ok {
		return fmt.Errorf("device %s not registered", deviceID)
	}
	// Deactivate previous
	if cd.active != "" {
		if prev, ok := cd.devices[cd.active]; ok {
			prev.IsActive = false
		}
	}
	cd.devices[deviceID].IsActive = true
	cd.active = deviceID
	return nil
}

// PrimaryDevice returns the currently active device.
func (cd *CrossDevice) PrimaryDevice() *DeviceProfile {
	cd.mu.RLock()
	defer cd.mu.RUnlock()
	return cd.devices[cd.active]
}

// Sync performs differential sync from a device.
func (cd *CrossDevice) Sync(deviceID string, blobs []ContextBlob) (*SyncResult, error) {
	cd.mu.Lock()
	defer cd.mu.Unlock()

	if _, ok := cd.devices[deviceID]; !ok {
		return nil, fmt.Errorf("device %s not registered", deviceID)
	}

	start := time.Now()
	result := &SyncResult{DeviceID: deviceID}

	for _, incoming := range blobs {
		existing, exists := cd.blobs[incoming.ID]

		if !exists {
			// New blob — accept
			incoming.Version = 1
			incoming.DeviceID = deviceID
			incoming.Modified = time.Now()
			incoming.Hash = incoming.ComputeHash()
			cd.blobs[incoming.ID] = &incoming
			cd.history[incoming.ID] = append(cd.history[incoming.ID], incoming)
			result.Uploaded++
			continue
		}

		if incoming.Version > existing.Version {
			// Newer version from device — accept
			incoming.DeviceID = deviceID
			incoming.Modified = time.Now()
			incoming.Hash = incoming.ComputeHash()
			cd.blobs[incoming.ID] = &incoming
			cd.history[incoming.ID] = append(cd.history[incoming.ID], incoming)
			result.Uploaded++
		} else if incoming.Version == existing.Version {
			// Same version — check if data differs
			incomingHash := incoming.ComputeHash()
			if incomingHash != existing.Hash {
				// Conflict!
				result.Conflicts = append(result.Conflicts, SyncConflict{
					BlobID:     incoming.ID,
					LocalVer:   existing.Version,
					RemoteVer:  incoming.Version,
					Resolution: ResolveLastWriterWins,
				})
				// Auto-resolve: last writer wins
				incoming.Version = existing.Version + 1
				incoming.DeviceID = deviceID
				incoming.Modified = time.Now()
				incoming.Hash = incomingHash
				cd.blobs[incoming.ID] = &incoming
				cd.history[incoming.ID] = append(cd.history[incoming.ID], incoming)
			}
		} else {
			// Device has older version — download
			result.Downloaded++
		}
	}

	cd.devices[deviceID].LastSync = time.Now()
	result.Duration = time.Since(start)
	return result, nil
}

// GetContext returns context blobs for a device (differential since last sync).
func (cd *CrossDevice) GetContext(deviceID string) ([]ContextBlob, error) {
	cd.mu.RLock()
	defer cd.mu.RUnlock()

	device, ok := cd.devices[deviceID]
	if !ok {
		return nil, fmt.Errorf("device %s not registered", deviceID)
	}

	// Return all blobs modified since device's last sync
	var blobs []ContextBlob
	for _, blob := range cd.blobs {
		if blob.Modified.After(device.LastSync) {
			blobs = append(blobs, *blob)
		}
	}
	return blobs, nil
}

// ResolveConflict manually resolves a sync conflict.
func (cd *CrossDevice) ResolveConflict(conflictID string, resolution ResolutionStrategy) error {
	cd.mu.Lock()
	defer cd.mu.Unlock()
	// In production, this would apply the chosen resolution strategy
	return nil
}
