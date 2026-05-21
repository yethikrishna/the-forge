package persistence_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/forge/sword/internal/persistence"
)

// ──────────────────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────────────────

func tempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "persistence-test-*")
	if err != nil {
		t.Fatalf("tempDir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

type counter struct {
	mu    sync.Mutex
	value int
}

func (c *counter) Inc() {
	c.mu.Lock()
	c.value++
	c.mu.Unlock()
}

func (c *counter) Get() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.value
}

func (c *counter) MarshalJSON() ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return json.Marshal(map[string]int{"value": c.value})
}

// ──────────────────────────────────────────────────────────────────────────────
// Tests
// ──────────────────────────────────────────────────────────────────────────────

func TestOpenCreatesDir(t *testing.T) {
	base := filepath.Join(os.TempDir(), "persistence-mkdir-test")
	defer os.RemoveAll(base)

	dir := filepath.Join(base, "deep", "nested", "dir")
	s, err := persistence.Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Fatal("Open did not create directory")
	}
}

func TestFlushWritesFile(t *testing.T) {
	dir := tempDir(t)
	c := &counter{}

	s, err := persistence.Open(dir, persistence.WithFlushInterval(10*time.Second))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	s.Register("mycounter", c.MarshalJSON)
	c.Inc()
	s.Dirty("mycounter")

	if err := s.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "mycounter.json"))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	var got map[string]int
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["value"] != 1 {
		t.Errorf("expected value=1, got %d", got["value"])
	}
}

func TestFlushMultipleKeys(t *testing.T) {
	dir := tempDir(t)
	a, b := &counter{}, &counter{}

	s, err := persistence.Open(dir, persistence.WithFlushInterval(10*time.Second))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	s.Register("a", a.MarshalJSON)
	s.Register("b", b.MarshalJSON)

	a.Inc()
	a.Inc()
	b.Inc()
	s.Dirty("a")
	s.Dirty("b")

	if err := s.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	for name, expected := range map[string]int{"a": 2, "b": 1} {
		data, _ := os.ReadFile(filepath.Join(dir, name+".json"))
		var got map[string]int
		json.Unmarshal(data, &got)
		if got["value"] != expected {
			t.Errorf("key %s: expected %d got %d", name, expected, got["value"])
		}
	}
}

func TestDirtyOnlyFlushes(t *testing.T) {
	dir := tempDir(t)
	a, b := &counter{}, &counter{}

	s, err := persistence.Open(dir, persistence.WithFlushInterval(10*time.Second))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	s.Register("a", a.MarshalJSON)
	s.Register("b", b.MarshalJSON)
	a.Inc()
	b.Inc()

	// Only mark "a" dirty.
	s.Dirty("a")
	if err := s.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "a.json")); os.IsNotExist(err) {
		t.Error("a.json should exist")
	}
	if _, err := os.Stat(filepath.Join(dir, "b.json")); !os.IsNotExist(err) {
		t.Error("b.json should NOT exist (not dirty)")
	}
}

func TestBackgroundFlush(t *testing.T) {
	dir := tempDir(t)
	c := &counter{}

	s, err := persistence.Open(dir, persistence.WithFlushInterval(50*time.Millisecond))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	s.Register("bg", c.MarshalJSON)
	c.Inc()
	s.Dirty("bg")

	// Wait for background flush.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
		if _, err := os.Stat(filepath.Join(dir, "bg.json")); err == nil {
			return // success
		}
	}
	t.Error("background flush did not write bg.json within 2s")
}

func TestWALReplay(t *testing.T) {
	dir := tempDir(t)

	// Simulate a crash: write a WAL file but no corresponding .json.
	walData := []byte(`{"recovered":true}`)
	walPath := filepath.Join(dir, "crash.wal")
	if err := os.WriteFile(walPath, walData, 0o644); err != nil {
		t.Fatal(err)
	}

	// Opening the store should replay the WAL.
	s, err := persistence.Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	data, err := os.ReadFile(filepath.Join(dir, "crash.json"))
	if err != nil {
		t.Fatalf("crash.json not created after WAL replay: %v", err)
	}
	var got map[string]interface{}
	json.Unmarshal(data, &got)
	if got["recovered"] != true {
		t.Errorf("expected recovered=true, got %v", got["recovered"])
	}

	// WAL file should be removed.
	if _, err := os.Stat(walPath); !os.IsNotExist(err) {
		t.Error("WAL file should be removed after replay")
	}
}

func TestWALReplayCorrupt(t *testing.T) {
	dir := tempDir(t)

	// Corrupt WAL — should be discarded, not promoted.
	walPath := filepath.Join(dir, "corrupt.wal")
	if err := os.WriteFile(walPath, []byte("not-json{{{"), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := persistence.Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	// corrupt.json should NOT exist.
	if _, err := os.Stat(filepath.Join(dir, "corrupt.json")); !os.IsNotExist(err) {
		t.Error("corrupt.json should not have been created")
	}
}

func TestCloseFlushesRemaining(t *testing.T) {
	dir := tempDir(t)
	c := &counter{}

	s, err := persistence.Open(dir, persistence.WithFlushInterval(10*time.Second))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	s.Register("close_test", c.MarshalJSON)
	c.Inc()
	s.Dirty("close_test")

	// Close should flush.
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "close_test.json")); os.IsNotExist(err) {
		t.Error("close_test.json should exist after Close")
	}
}

func TestConcurrentDirty(t *testing.T) {
	dir := tempDir(t)
	c := &counter{}

	s, err := persistence.Open(dir, persistence.WithFlushInterval(10*time.Second))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()
	s.Register("concurrent", c.MarshalJSON)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Inc()
			s.Dirty("concurrent")
		}()
	}
	wg.Wait()

	if err := s.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "concurrent.json"))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	var got map[string]int
	json.Unmarshal(data, &got)
	if got["value"] != 100 {
		t.Errorf("expected 100, got %d", got["value"])
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Benchmarks
// ──────────────────────────────────────────────────────────────────────────────

func BenchmarkDirty(b *testing.B) {
	dir := b.TempDir()
	c := &counter{}

	s, _ := persistence.Open(dir, persistence.WithFlushInterval(10*time.Second))
	defer s.Close()
	s.Register("bench", c.MarshalJSON)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		c.Inc()
		s.Dirty("bench")
	}
}

func BenchmarkFlush(b *testing.B) {
	dir := b.TempDir()
	c := &counter{}

	s, _ := persistence.Open(dir, persistence.WithFlushInterval(10*time.Second))
	defer s.Close()
	s.Register("bench", c.MarshalJSON)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		c.Inc()
		s.Dirty("bench")
		s.Flush()
	}
}
