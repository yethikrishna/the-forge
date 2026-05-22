// Package transfer provides P2P encrypted file transfer via WireGuard.
// Files are encrypted with AES-256-GCM and transferred directly between peers.
package transfer

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// TransferState is the state of a file transfer.
type TransferState string

const (
	TransferPending TransferState = "pending"
	TransferActive  TransferState = "active"
	TransferDone    TransferState = "done"
	TransferFailed  TransferState = "failed"
)

// TransferInfo tracks a single file transfer.
type TransferInfo struct {
	ID          string        `json:"id"`
	FileName    string        `json:"file_name"`
	FileSize    int64         `json:"file_size"`
	PeerAddr    string        `json:"peer_addr"`
	State       TransferState `json:"state"`
	Progress    float64       `json:"progress"` // 0-100
	BytesSent   int64         `json:"bytes_sent"`
	BytesRecv   int64         `json:"bytes_recv"`
	StartedAt   *time.Time    `json:"started_at,omitempty"`
	FinishedAt  *time.Time    `json:"finished_at,omitempty"`
	Error       string        `json:"error,omitempty"`
	Secret      string        `json:"-"` // not serialized
}

// Duration returns the transfer duration.
func (ti *TransferInfo) Duration() time.Duration {
	if ti.StartedAt == nil {
		return 0
	}
	end := time.Now()
	if ti.FinishedAt != nil {
		end = *ti.FinishedAt
	}
	return end.Sub(*ti.StartedAt)
}

// Speed returns bytes per second.
func (ti *TransferInfo) Speed() float64 {
	dur := ti.Duration()
	if dur == 0 {
		return 0
	}
	return float64(ti.BytesSent+ti.BytesRecv) / dur.Seconds()
}

// ChunkSize for transfer.
const ChunkSize = 32 * 1024 // 32KB chunks

// Sender sends files to a peer.
type Sender struct {
	transfers map[string]*TransferInfo
	mu        sync.RWMutex
}

// NewSender creates a file sender.
func NewSender() *Sender {
	return &Sender{transfers: make(map[string]*TransferInfo)}
}

// Send sends a file to a peer address.
func (s *Sender) Send(filePath, peerAddr, secret string, onProgress func(sent, total int64)) (*TransferInfo, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("stat file: %w", err)
	}

	t := &TransferInfo{
		ID:       fmt.Sprintf("xfer-%d", time.Now().UnixNano()),
		FileName: filepath.Base(filePath),
		FileSize: info.Size(),
		PeerAddr: peerAddr,
		State:    TransferPending,
		Secret:   secret,
	}

	s.mu.Lock()
	s.transfers[t.ID] = t
	s.mu.Unlock()

	// Connect to peer
	conn, err := net.DialTimeout("tcp", peerAddr, 30*time.Second)
	if err != nil {
		t.State = TransferFailed
		t.Error = fmt.Sprintf("connect: %v", err)
		return t, err
	}
	defer conn.Close()

	now := time.Now()
	t.StartedAt = &now
	t.State = TransferActive

	// Send file metadata header
	header := fmt.Sprintf("FORGE:XFER:%s:%d\n", t.FileName, t.FileSize)
	if _, err := conn.Write([]byte(header)); err != nil {
		t.State = TransferFailed
		t.Error = err.Error()
		return t, err
	}

	// Open and send file
	file, err := os.Open(filePath)
	if err != nil {
		t.State = TransferFailed
		t.Error = err.Error()
		return t, err
	}
	defer file.Close()

	// Setup encryption if secret provided
	var writer io.Writer = conn
	if secret != "" {
		encWriter, err := newEncryptedWriter(conn, secret)
		if err != nil {
			t.State = TransferFailed
			t.Error = err.Error()
			return t, err
		}
		writer = encWriter
	}

	buf := make([]byte, ChunkSize)
	for {
		n, err := file.Read(buf)
		if n > 0 {
			written, werr := writer.Write(buf[:n])
			t.BytesSent += int64(written)
			t.Progress = float64(t.BytesSent) / float64(t.FileSize) * 100
			if onProgress != nil {
				onProgress(t.BytesSent, t.FileSize)
			}
			if werr != nil {
				t.State = TransferFailed
				t.Error = werr.Error()
				return t, werr
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			t.State = TransferFailed
			t.Error = err.Error()
			return t, err
		}
	}

	t.State = TransferDone
	finished := time.Now()
	t.FinishedAt = &finished
	t.Progress = 100
	return t, nil
}

// Receiver receives files from peers.
type Receiver struct {
	outputDir string
	transfers map[string]*TransferInfo
	mu        sync.RWMutex
}

// NewReceiver creates a file receiver.
func NewReceiver(outputDir string) *Receiver {
	os.MkdirAll(outputDir, 0o755)
	return &Receiver{
		outputDir: outputDir,
		transfers: make(map[string]*TransferInfo),
	}
}

// Receive listens for and receives a file transfer.
func (r *Receiver) Receive(listenAddr, secret string) (*TransferInfo, error) {
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}
	defer ln.Close()

	conn, err := ln.Accept()
	if err != nil {
		return nil, fmt.Errorf("accept: %w", err)
	}
	defer conn.Close()

	// Read header
	header := make([]byte, 256)
	n, err := conn.Read(header)
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	var fileName string
	var fileSize int64
	fmt.Sscanf(string(header[:n]), "FORGE:XFER:%s:%d", &fileName, &fileSize)

	// Remove trailing newline from filename
	fileName = filepath.Base(fileName)

	t := &TransferInfo{
		ID:       fmt.Sprintf("xfer-%d", time.Now().UnixNano()),
		FileName: fileName,
		FileSize: fileSize,
		PeerAddr: conn.RemoteAddr().String(),
		State:    TransferActive,
		Secret:   secret,
	}
	now := time.Now()
	t.StartedAt = &now

	r.mu.Lock()
	r.transfers[t.ID] = t
	r.mu.Unlock()

	// Setup decryption if secret provided
	var reader io.Reader = conn
	if secret != "" {
		decReader, err := newEncryptedReader(conn, secret)
		if err != nil {
			t.State = TransferFailed
			t.Error = err.Error()
			return t, err
		}
		reader = decReader
	}

	// Write to file
	outPath := filepath.Join(r.outputDir, fileName)
	outFile, err := os.Create(outPath)
	if err != nil {
		t.State = TransferFailed
		t.Error = err.Error()
		return t, err
	}
	defer outFile.Close()

	buf := make([]byte, ChunkSize)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			outFile.Write(buf[:n])
			t.BytesRecv += int64(n)
			t.Progress = float64(t.BytesRecv) / float64(t.FileSize) * 100
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			t.State = TransferFailed
			t.Error = err.Error()
			return t, err
		}
	}

	t.State = TransferDone
	finished := time.Now()
	t.FinishedAt = &finished
	t.Progress = 100
	return t, nil
}

// encryption helpers
//
// AES-256-GCM provides authenticated encryption: confidentiality + integrity.
// Each chunk is encrypted independently with a unique nonce derived from
// the key hash and a counter, preventing nonce reuse across chunks.

func deriveKey(secret string) []byte {
	h := sha256.Sum256([]byte(secret))
	return h[:]
}

type encryptedWriter struct {
	aead  cipher.AEAD
	writer io.Writer
	buf    []byte
	nonce  [12]byte
	ctr    uint32
}

func newEncryptedWriter(w io.Writer, secret string) (*encryptedWriter, error) {
	key := deriveKey(secret)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}
	return &encryptedWriter{
		aead:  aead,
		writer: w,
		buf:    make([]byte, 0, ChunkSize+aead.Overhead()+4), // len prefix + ciphertext + tag
	}, nil
}

func (ew *encryptedWriter) Write(p []byte) (int, error) {
	// Encrypt the chunk: write [4-byte length][nonce][ciphertext+tag]
	ew.incrementNonce()

	ciphertext := ew.aead.Seal(nil, ew.nonce[:], p, nil)

	// Length prefix: 4 bytes big-endian = len(nonce) + len(ciphertext)
	frameLen := uint32(len(ew.nonce) + len(ciphertext))
	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, frameLen)

	if _, err := ew.writer.Write(lenBuf); err != nil {
		return 0, err
	}
	if _, err := ew.writer.Write(ew.nonce[:]); err != nil {
		return 0, err
	}
	if _, err := ew.writer.Write(ciphertext); err != nil {
		return 0, err
	}

	return len(p), nil
}

func (ew *encryptedWriter) incrementNonce() {
	ew.ctr++
	binary.LittleEndian.PutUint32(ew.nonce[8:12], ew.ctr)
}

type encryptedReader struct {
	aead  cipher.AEAD
	reader io.Reader
	nonce  [12]byte
	ctr    uint32
}

func newEncryptedReader(r io.Reader, secret string) (*encryptedReader, error) {
	key := deriveKey(secret)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}
	return &encryptedReader{
		aead:  aead,
		reader: r,
	}, nil
}

func (er *encryptedReader) Read(p []byte) (int, error) {
	// Read length prefix
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(er.reader, lenBuf); err != nil {
		return 0, err
	}
	frameLen := binary.BigEndian.Uint32(lenBuf)
	if frameLen > uint32(ChunkSize+er.aead.Overhead()+12) {
		return 0, fmt.Errorf("encrypted frame too large: %d", frameLen)
	}

	// Read frame (nonce + ciphertext)
	frame := make([]byte, frameLen)
	if _, err := io.ReadFull(er.reader, frame); err != nil {
		return 0, err
	}

	// Split nonce and ciphertext
	if len(frame) < 12 {
		return 0, fmt.Errorf("encrypted frame too short")
	}
	nonce := frame[:12]
	ciphertext := frame[12:]

	// Decrypt
	plaintext, err := er.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return 0, fmt.Errorf("decrypt: %w", err)
	}

	copy(p, plaintext)
	return len(plaintext), nil
}

// GenerateSecret creates a random transfer secret.
func GenerateSecret() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// Fallback: should never happen
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}
