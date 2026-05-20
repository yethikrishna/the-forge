package wush_test

import (
	"testing"

	"github.com/forge/sword/internal/wush"
)

func TestNewManager(t *testing.T) {
	mgr := wush.NewManager()
	if mgr == nil {
		t.Fatal("manager should not be nil")
	}
}

func TestListEmpty(t *testing.T) {
	mgr := wush.NewManager()
	list := mgr.List()
	if len(list) != 0 {
		t.Errorf("expected empty list, got %d", len(list))
	}
}

func TestGetNonExistent(t *testing.T) {
	mgr := wush.NewManager()
	_, err := mgr.Get("nonexistent")
	if err == nil {
		t.Error("should error for non-existent transfer")
	}
}

func TestDefaultTransferConfig(t *testing.T) {
	cfg := wush.DefaultTransferConfig()
	if cfg.ChunkSize != 64*1024 {
		t.Errorf("expected 64KB chunk size, got %d", cfg.ChunkSize)
	}
}

func TestTransferProgress(t *testing.T) {
	transfer := &wush.Transfer{
		FileSize:         1000,
		BytesTransferred: 500,
	}
	if transfer.Progress() != 50.0 {
		t.Errorf("expected 50%%, got %.1f%%", transfer.Progress())
	}
}

func TestTransferProgressZero(t *testing.T) {
	transfer := &wush.Transfer{FileSize: 0}
	if transfer.Progress() != 0 {
		t.Errorf("expected 0%%, got %.1f%%", transfer.Progress())
	}
}

func TestChecksum(t *testing.T) {
	// Checksum of a non-existent file should error
	_, err := wush.Checksum("/tmp/nonexistent-file-xyz-12345")
	if err == nil {
		t.Error("should error for non-existent file")
	}
}
