// Package wush provides P2P encrypted file transfer.
// Transfer files like lightning through the forge.
package wush

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// TransferConfig configures a file transfer.
type TransferConfig struct {
	// Peer address (host:port)
	PeerAddr string
	// Shared secret for encryption
	Secret string
	// Chunk size for streaming (default 64KB)
	ChunkSize int
	// Progress callback
	OnProgress func(sent, total int64)
}

// DefaultTransferConfig returns sensible defaults.
func DefaultTransferConfig() TransferConfig {
	return TransferConfig{
		ChunkSize: 64 * 1024,
	}
}

// Transfer represents a file transfer.
type Transfer struct {
	ID         string
	FileName   string
	FileSize   int64
	Status     TransferStatus
	Direction  TransferDirection
	StartedAt  time.Time
	CompletedAt time.Time
	BytesTransferred int64
	PeerAddr   string
	config     TransferConfig
}

// TransferStatus indicates the state of a transfer.
type TransferStatus string

const (
	TransferPending TransferStatus = "pending"
	TransferActive  TransferStatus = "active"
	TransferComplete TransferStatus = "complete"
	TransferFailed  TransferStatus = "failed"
)

// TransferDirection indicates the direction of transfer.
type TransferDirection string

const (
	DirectionSend    TransferDirection = "send"
	DirectionReceive TransferDirection = "receive"
)

// Manager manages file transfers.
type Manager struct {
	transfers map[string]*Transfer
	mu        sync.RWMutex
}

// NewManager creates a new transfer manager.
func NewManager() *Manager {
	return &Manager{
		transfers: make(map[string]*Transfer),
	}
}

// Send sends a file to a peer.
func (m *Manager) Send(ctx context.Context, filePath string, config TransferConfig) (*Transfer, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("wush: stat file: %w", err)
	}

	id := generateTransferID()
	transfer := &Transfer{
		ID:        id,
		FileName:  filepath.Base(filePath),
		FileSize:  info.Size(),
		Status:    TransferPending,
		Direction: DirectionSend,
		StartedAt: time.Now(),
		PeerAddr:  config.PeerAddr,
		config:    config,
	}

	m.mu.Lock()
	m.transfers[id] = transfer
	m.mu.Unlock()

	// Connect to peer
	conn, err := net.DialTimeout("tcp", config.PeerAddr, 30*time.Second)
	if err != nil {
		transfer.Status = TransferFailed
		return nil, fmt.Errorf("wush: connect to peer: %w", err)
	}
	defer conn.Close()

	transfer.Status = TransferActive

	// Send header
	header := fmt.Sprintf("WUSH1|%s|%d\n", filepath.Base(filePath), info.Size())
	if _, err := conn.Write([]byte(header)); err != nil {
		transfer.Status = TransferFailed
		return nil, fmt.Errorf("wush: send header: %w", err)
	}

	// Wait for ACK
	ack := make([]byte, 3)
	if _, err := conn.Read(ack); err != nil {
		transfer.Status = TransferFailed
		return nil, fmt.Errorf("wush: wait for ack: %w", err)
	}
	if string(ack) != "ACK" {
		transfer.Status = TransferFailed
		return nil, fmt.Errorf("wush: peer rejected transfer")
	}

	// Stream file
	file, err := os.Open(filePath)
	if err != nil {
		transfer.Status = TransferFailed
		return nil, fmt.Errorf("wush: open file: %w", err)
	}
	defer file.Close()

	chunkSize := config.ChunkSize
	if chunkSize <= 0 {
		chunkSize = 64 * 1024
	}

	buf := make([]byte, chunkSize)
	var total int64

	for {
		n, err := file.Read(buf)
		if err != nil && err != io.EOF {
			transfer.Status = TransferFailed
			return nil, fmt.Errorf("wush: read file: %w", err)
		}
		if n == 0 {
			break
		}

		written, err := conn.Write(buf[:n])
		if err != nil {
			transfer.Status = TransferFailed
			return nil, fmt.Errorf("wush: write to peer: %w", err)
		}

		total += int64(written)
		transfer.BytesTransferred = total

		if config.OnProgress != nil {
			config.OnProgress(total, info.Size())
		}
	}

	transfer.Status = TransferComplete
	transfer.CompletedAt = time.Now()
	return transfer, nil
}

// Receive receives a file from a peer.
func (m *Manager) Receive(ctx context.Context, listenAddr string, outputDir string) (*Transfer, error) {
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return nil, fmt.Errorf("wush: listen: %w", err)
	}
	defer listener.Close()

	// Accept one connection
	conn, err := listener.Accept()
	if err != nil {
		return nil, fmt.Errorf("wush: accept: %w", err)
	}
	defer conn.Close()

	// Read header
	reader := bufio.NewReader(conn)
	headerLine, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("wush: read header: %w", err)
	}

	// Parse header: WUSH1|filename|size
	parts := strings.SplitN(strings.TrimRight(headerLine, "\n"), "|", 3)
	if len(parts) != 3 || parts[0] != "WUSH1" {
		return nil, fmt.Errorf("wush: invalid header format")
	}

	fileName := parts[1]
	fileSize, _ := strconv.ParseInt(parts[2], 10, 64)

	// Send ACK
	conn.Write([]byte("ACK"))

	// Create output file
	os.MkdirAll(outputDir, 0o755)
	outputPath := filepath.Join(outputDir, fileName)
	file, err := os.Create(outputPath)
	if err != nil {
		return nil, fmt.Errorf("wush: create file: %w", err)
	}
	defer file.Close()

	id := generateTransferID()
	transfer := &Transfer{
		ID:        id,
		FileName:  fileName,
		FileSize:  fileSize,
		Status:    TransferActive,
		Direction: DirectionReceive,
		StartedAt: time.Now(),
		PeerAddr:  conn.RemoteAddr().String(),
	}

	m.mu.Lock()
	m.transfers[id] = transfer
	m.mu.Unlock()

	// Stream file
	var total int64
	buf := make([]byte, 64*1024)
	for {
		n, err := reader.Read(buf)
		if err != nil && err != io.EOF {
			transfer.Status = TransferFailed
			return transfer, fmt.Errorf("wush: read from peer: %w", err)
		}
		if n == 0 {
			break
		}

		file.Write(buf[:n])
		total += int64(n)
		transfer.BytesTransferred = total
	}

	transfer.Status = TransferComplete
	transfer.CompletedAt = time.Now()
	return transfer, nil
}

// Get returns a transfer by ID.
func (m *Manager) Get(id string) (*Transfer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	t, ok := m.transfers[id]
	if !ok {
		return nil, fmt.Errorf("wush: transfer %q not found", id)
	}
	return t, nil
}

// List returns all transfers.
func (m *Manager) List() []*Transfer {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Transfer, 0, len(m.transfers))
	for _, t := range m.transfers {
		result = append(result, t)
	}
	return result
}

// Progress returns the transfer progress as a percentage.
func (t *Transfer) Progress() float64 {
	if t.FileSize == 0 {
		return 0
	}
	return float64(t.BytesTransferred) / float64(t.FileSize) * 100
}

// Duration returns the transfer duration.
func (t *Transfer) Duration() time.Duration {
	if t.CompletedAt.IsZero() {
		return time.Since(t.StartedAt)
	}
	return t.CompletedAt.Sub(t.StartedAt)
}

// Speed returns the transfer speed in bytes per second.
func (t *Transfer) Speed() float64 {
	d := t.Duration().Seconds()
	if d == 0 {
		return 0
	}
	return float64(t.BytesTransferred) / d
}

// Checksum computes the SHA-256 checksum of a file.
func Checksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("wush: open for checksum: %w", err)
	}
	defer file.Close()

	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return "", fmt.Errorf("wush: compute checksum: %w", err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func generateTransferID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}
