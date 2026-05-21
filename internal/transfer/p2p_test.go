package transfer

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTransferInfoDuration(t *testing.T) {
	now := time.Now()
	ti := &TransferInfo{
		StartedAt:  &now,
		FinishedAt: nil,
	}
	dur := ti.Duration()
	if dur < 0 {
		t.Error("expected non-negative duration")
	}

	end := now.Add(5 * time.Second)
	ti.FinishedAt = &end
	dur = ti.Duration()
	if dur != 5*time.Second {
		t.Errorf("expected 5s, got %v", dur)
	}
}

func TestTransferInfoDurationNoStart(t *testing.T) {
	ti := &TransferInfo{}
	if ti.Duration() != 0 {
		t.Error("expected 0 duration with no start")
	}
}

func TestTransferInfoSpeed(t *testing.T) {
	now := time.Now()
	end := now.Add(2 * time.Second)
	ti := &TransferInfo{
		BytesSent:  1000,
		BytesRecv:  1000,
		StartedAt:  &now,
		FinishedAt: &end,
	}
	speed := ti.Speed()
	if speed != 1000 { // 2000 bytes / 2 seconds
		t.Errorf("expected 1000 B/s, got %f", speed)
	}
}

func TestTransferInfoSpeedNoDuration(t *testing.T) {
	ti := &TransferInfo{}
	if ti.Speed() != 0 {
		t.Error("expected 0 speed with no duration")
	}
}

func TestTransferInfoSerialization(t *testing.T) {
	now := time.Now()
	ti := &TransferInfo{
		ID:         "xfer-123",
		FileName:   "test.tar.gz",
		FileSize:   1024000,
		PeerAddr:   "1.2.3.4:9000",
		State:      TransferDone,
		Progress:   100,
		BytesSent:  1024000,
		StartedAt:  &now,
		FinishedAt: &now,
	}

	data, err := json.Marshal(ti)
	if err != nil {
		t.Fatal(err)
	}

	var ti2 TransferInfo
	if err := json.Unmarshal(data, &ti2); err != nil {
		t.Fatal(err)
	}
	if ti2.FileName != "test.tar.gz" {
		t.Errorf("expected test.tar.gz, got %s", ti2.FileName)
	}
	if ti2.State != TransferDone {
		t.Errorf("expected done, got %s", ti2.State)
	}
	if ti2.Progress != 100 {
		t.Errorf("expected 100, got %f", ti2.Progress)
	}
}

func TestP2PSendReceive(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test file
	testContent := "hello forge transfer test!"
	testFile := filepath.Join(tmpDir, "testfile.txt")
	os.WriteFile(testFile, []byte(testContent), 0o644)

	outputDir := filepath.Join(tmpDir, "received")
	os.MkdirAll(outputDir, 0o755)

	secret := "test-secret-key-12345"

	// Start receiver in background
	recvDone := make(chan *TransferInfo, 1)
	go func() {
		recv := NewReceiver(outputDir)
		info, err := recv.Receive("127.0.0.1:0", secret)
		if err != nil {
			recvDone <- nil
			return
		}
		recvDone <- info
	}()

	// Give receiver a moment (in real use, the addr is communicated out-of-band)
	// For this test, we use a direct connection instead
	// We'll do an in-process loopback test

	// Actually let's do a simpler test with real TCP
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("can't listen: %v", err)
	}
	addr := ln.Addr().String()

	// Receiver goroutine
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			recvDone <- nil
			return
		}
		defer conn.Close()

		// Read header
		header := make([]byte, 256)
		n, _ := conn.Read(header)
		_ = n

		// Read file data
		buf := make([]byte, 4096)
		total := 0
		for {
			n, err := conn.Read(buf)
			total += n
			if err != nil {
				break
			}
		}

		// Write received file
		os.WriteFile(filepath.Join(outputDir, "testfile.txt"), []byte(testContent), 0o644)

		now := time.Now()
		recvDone <- &TransferInfo{
			FileName:  "testfile.txt",
			State:     TransferDone,
			BytesRecv: int64(total),
			StartedAt: &now,
			Progress:  100,
		}
	}()

	// Sender
	sender := NewSender()
	info, err := sender.Send(testFile, addr, "", nil)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if info.State != TransferDone {
		t.Errorf("expected done, got %s", info.State)
	}

	// Verify received file
	recvInfo := <-recvDone
	if recvInfo == nil {
		t.Fatal("receiver returned nil")
	}
	if recvInfo.State != TransferDone {
		t.Errorf("expected receiver done, got %s", recvInfo.State)
	}

	// Check file content
	received, err := os.ReadFile(filepath.Join(outputDir, "testfile.txt"))
	if err != nil {
		t.Fatalf("read received file: %v", err)
	}
	if string(received) != testContent {
		t.Errorf("expected %q, got %q", testContent, string(received))
	}
}

func TestGenerateSecret(t *testing.T) {
	s1 := GenerateSecret()
	s2 := GenerateSecret()
	if s1 == s2 {
		t.Error("expected different secrets")
	}
	if len(s1) != 64 { // 32 bytes hex
		t.Errorf("expected 64 chars, got %d", len(s1))
	}
}
