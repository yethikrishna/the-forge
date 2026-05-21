package openclaw

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewBridge(t *testing.T) {
	cfg := BridgeConfig{
		GatewayURL:   "http://localhost:3271",
		WorkspaceDir: "/tmp/test-workspace",
	}
	b, err := NewBridge(cfg)
	if err != nil {
		t.Fatalf("NewBridge: %v", err)
	}
	defer b.Close()
	if b.GatewayURL() != "http://localhost:3271" {
		t.Errorf("expected localhost:3271, got %s", b.GatewayURL())
	}
	if b.WorkspaceDir() != "/tmp/test-workspace" {
		t.Errorf("expected /tmp/test-workspace, got %s", b.WorkspaceDir())
	}
}

func TestBridgeHealth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	b, _ := NewBridge(BridgeConfig{GatewayURL: srv.URL})
	defer b.Close()

	if err := b.Health(context.Background()); err != nil {
		t.Fatalf("health check failed: %v", err)
	}
}

func TestBridgeGetJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/test" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"key": "value"})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	b, _ := NewBridge(BridgeConfig{GatewayURL: srv.URL})
	defer b.Close()

	var result map[string]string
	if err := b.GetJSON(context.Background(), "/api/test", &result); err != nil {
		t.Fatalf("GetJSON: %v", err)
	}
	if result["key"] != "value" {
		t.Errorf("expected value, got %v", result)
	}
}

func TestBridgePostJSON(t *testing.T) {
	var receivedBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/api/create" {
			json.NewDecoder(r.Body).Decode(&receivedBody)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"id": "123", "status": "created"})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	b, _ := NewBridge(BridgeConfig{GatewayURL: srv.URL})
	defer b.Close()

	var result map[string]interface{}
	err := b.PostJSON(context.Background(), "/api/create", map[string]string{"name": "test"}, &result)
	if err != nil {
		t.Fatalf("PostJSON: %v", err)
	}
	if receivedBody["name"] != "test" {
		t.Errorf("expected name=test in body, got %v", receivedBody)
	}
	if result["id"] != "123" {
		t.Errorf("expected id=123, got %v", result)
	}
}

func TestBridgeAuth(t *testing.T) {
	var authHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	b, _ := NewBridge(BridgeConfig{
		GatewayURL:   srv.URL,
		GatewayToken: "test-token-123",
	})
	defer b.Close()

	b.GetJSON(context.Background(), "/api/test", nil)
	if authHeader != "Bearer test-token-123" {
		t.Errorf("expected Bearer token, got %s", authHeader)
	}
}

func TestBridgeClosed(t *testing.T) {
	b, _ := NewBridge(BridgeConfig{GatewayURL: "http://localhost:1"})
	b.Close()
	err := b.Health(context.Background())
	if err == nil {
		t.Error("expected error on closed bridge")
	}
}

func TestBridgeErrorHandling(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	b, _ := NewBridge(BridgeConfig{GatewayURL: srv.URL})
	defer b.Close()

	err := b.GetJSON(context.Background(), "/api/test", nil)
	if err == nil {
		t.Error("expected error on 500 response")
	}
}

// CronManager tests

func TestCronCreate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/api/cron" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":       "test-cron",
				"next_run": time.Now().Add(time.Hour).Format(time.RFC3339),
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	b, _ := NewBridge(BridgeConfig{GatewayURL: srv.URL})
	defer b.Close()
	cm := NewCronManager(b)

	schedule, err := cm.Create(context.Background(), CronEntry{
		Expression: "*/5 * * * *",
		Task:       "heartbeat",
		Label:      "test-cron",
		Division:   "engineering",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if schedule.ID != "test-cron" {
		t.Errorf("expected test-cron, got %s", schedule.ID)
	}
	if !schedule.Enabled {
		t.Error("expected schedule to be enabled")
	}
}

func TestCronCreateValidation(t *testing.T) {
	b, _ := NewBridge(BridgeConfig{GatewayURL: "http://localhost:1"})
	defer b.Close()
	cm := NewCronManager(b)

	_, err := cm.Create(context.Background(), CronEntry{})
	if err == nil {
		t.Error("expected error for empty expression")
	}

	_, err = cm.Create(context.Background(), CronEntry{Expression: "*/5 * * * *"})
	if err == nil {
		t.Error("expected error for empty task")
	}
}

func TestCronList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/api/cron" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]*CronSchedule{
				{ID: "cron-1", Expression: "*/5 * * * *", Division: "engineering"},
				{ID: "cron-2", Expression: "0 * * * *", Division: "operations"},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	b, _ := NewBridge(BridgeConfig{GatewayURL: srv.URL})
	defer b.Close()
	cm := NewCronManager(b)

	jobs, err := cm.List(context.Background(), "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(jobs) != 2 {
		t.Errorf("expected 2 jobs, got %d", len(jobs))
	}
}

// SessionManager tests

func TestSessionCreate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/api/sessions" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(Session{
				ID:        "sess-123",
				Key:       "sess-123",
				AgentID:   "agent-1",
				Division:  "engineering",
				State:     SessionActive,
				CreatedAt: time.Now(),
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	b, _ := NewBridge(BridgeConfig{GatewayURL: srv.URL})
	defer b.Close()
	sm := NewSessionManager(b)

	sess, err := sm.Create(context.Background(), SessionCreate{
		AgentID:  "agent-1",
		Division: "engineering",
		Label:    "test session",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if sess.ID != "sess-123" {
		t.Errorf("expected sess-123, got %s", sess.ID)
	}
	if sess.State != SessionActive {
		t.Errorf("expected active, got %s", sess.State)
	}
}

func TestSessionCreateValidation(t *testing.T) {
	b, _ := NewBridge(BridgeConfig{GatewayURL: "http://localhost:1"})
	defer b.Close()
	sm := NewSessionManager(b)

	_, err := sm.Create(context.Background(), SessionCreate{})
	if err == nil {
		t.Error("expected error for empty agent_id")
	}
}

func TestSessionBranch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "POST" && r.URL.Path == "/api/sessions":
			json.NewEncoder(w).Encode(Session{
				ID: "sess-parent", AgentID: "agent-1", State: SessionActive,
			})
		case r.Method == "POST" && r.URL.Path == "/api/sessions/sess-parent/branch":
			json.NewEncoder(w).Encode(Session{
				ID: "sess-child", ParentID: "sess-parent", State: SessionActive,
			})
		}
	}))
	defer srv.Close()

	b, _ := NewBridge(BridgeConfig{GatewayURL: srv.URL})
	defer b.Close()
	sm := NewSessionManager(b)

	parent, _ := sm.Create(context.Background(), SessionCreate{AgentID: "agent-1"})
	child, err := sm.Branch(context.Background(), parent.ID, "branched")
	if err != nil {
		t.Fatalf("Branch: %v", err)
	}
	if child.ParentID != "sess-parent" {
		t.Errorf("expected parent sess-parent, got %s", child.ParentID)
	}
}

// ChannelManager tests

func TestChannelSend(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/api/message" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(Message{
				ID:        "msg-123",
				Channel:   ChannelSlack,
				Content:   "Hello",
				Timestamp: time.Now(),
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	b, _ := NewBridge(BridgeConfig{GatewayURL: srv.URL})
	defer b.Close()
	cm := NewChannelManager(b)

	msg, err := cm.Send(context.Background(), SendOptions{
		Channel: ChannelSlack,
		Target:  "#general",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if msg.ID != "msg-123" {
		t.Errorf("expected msg-123, got %s", msg.ID)
	}
}

func TestChannelSendValidation(t *testing.T) {
	b, _ := NewBridge(BridgeConfig{GatewayURL: "http://localhost:1"})
	defer b.Close()
	cm := NewChannelManager(b)

	_, err := cm.Send(context.Background(), SendOptions{})
	if err == nil {
		t.Error("expected error for empty channel")
	}

	_, err = cm.Send(context.Background(), SendOptions{Channel: ChannelSlack})
	if err == nil {
		t.Error("expected error for empty target")
	}
}

func TestChannelDivisionChannel(t *testing.T) {
	ch := DivisionChannel("engineering")
	if ch != "#forge-engineering" {
		t.Errorf("expected #forge-engineering, got %s", ch)
	}
}

// NodeManager tests

func TestNodeList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/api/nodes" {
			json.NewEncoder(w).Encode([]*Node{
				{ID: "node-1", Name: "laptop", Status: NodeOnline, OS: "linux"},
				{ID: "node-2", Name: "phone", Status: NodeOnline, OS: "android"},
				{ID: "node-3", Name: "server", Status: NodeOffline, OS: "linux"},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	b, _ := NewBridge(BridgeConfig{GatewayURL: srv.URL})
	defer b.Close()
	nm := NewNodeManager(b)

	nodes, err := nm.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(nodes) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(nodes))
	}
}

func TestNodeOnlineFilter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]*Node{
			{ID: "node-1", Status: NodeOnline},
			{ID: "node-2", Status: NodeOffline},
			{ID: "node-3", Status: NodeOnline},
		})
	}))
	defer srv.Close()

	b, _ := NewBridge(BridgeConfig{GatewayURL: srv.URL})
	defer b.Close()
	nm := NewNodeManager(b)

	online, err := nm.OnlineNodes(context.Background())
	if err != nil {
		t.Fatalf("OnlineNodes: %v", err)
	}
	if len(online) != 2 {
		t.Errorf("expected 2 online, got %d", len(online))
	}
}

func TestNodeCapability(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]*Node{
			{ID: "node-1", Status: NodeOnline, Capabilities: []string{"browser", "docker"}},
			{ID: "node-2", Status: NodeOnline, Capabilities: []string{"gpu"}},
		})
	}))
	defer srv.Close()

	b, _ := NewBridge(BridgeConfig{GatewayURL: srv.URL})
	defer b.Close()
	nm := NewNodeManager(b)

	ok, node, err := nm.HasCapability(context.Background(), "gpu")
	if err != nil {
		t.Fatalf("HasCapability: %v", err)
	}
	if !ok {
		t.Error("expected to find GPU capability")
	}
	if node.ID != "node-2" {
		t.Errorf("expected node-2, got %s", node.ID)
	}

	ok, _, _ = nm.HasCapability(context.Background(), "nonexistent")
	if ok {
		t.Error("expected no match for nonexistent capability")
	}
}

// MemoryManager tests

func TestMemoryStoreAndRetrieve(t *testing.T) {
	tmpDir := t.TempDir()
	b, _ := NewBridge(BridgeConfig{
		GatewayURL:   "http://localhost:1", // intentionally unreachable
		WorkspaceDir: tmpDir,
	})
	defer b.Close()
	mm := NewMemoryManager(b)

	entry := MemoryEntry{
		Type:    MemoryProject,
		Key:     "test/architecture",
		Content: "The system uses microservices architecture.",
		AgentID: "agent-1",
	}

	// Store should fall back to local
	if err := mm.Store(context.Background(), entry); err != nil {
		t.Fatalf("Store: %v", err)
	}

	// Retrieve should find it locally
	got, err := mm.Retrieve(context.Background(), "test/architecture")
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if got.Key != "test/architecture" {
		t.Errorf("expected test/architecture, got %s", got.Key)
	}
}

func TestMemoryDailyLog(t *testing.T) {
	tmpDir := t.TempDir()
	b, _ := NewBridge(BridgeConfig{WorkspaceDir: tmpDir})
	defer b.Close()
	mm := NewMemoryManager(b)

	if err := mm.AppendToDaily("2026-05-21", "Deployed v1.2.3 to production"); err != nil {
		t.Fatalf("AppendToDaily: %v", err)
	}
	if err := mm.AppendToDaily("2026-05-21", "Fixed critical bug in payment flow"); err != nil {
		t.Fatalf("AppendToDaily: %v", err)
	}

	content, err := mm.ReadDaily("2026-05-21")
	if err != nil {
		t.Fatalf("ReadDaily: %v", err)
	}
	if content == "" {
		t.Error("expected non-empty daily content")
	}
}

func TestMemoryLocalSearch(t *testing.T) {
	tmpDir := t.TempDir()
	b, _ := NewBridge(BridgeConfig{
		GatewayURL:   "http://localhost:1",
		WorkspaceDir: tmpDir,
	})
	defer b.Close()
	mm := NewMemoryManager(b)

	// Store some entries
	mm.Store(context.Background(), MemoryEntry{Key: "go-patterns", Content: "Go concurrency patterns"})
	mm.Store(context.Background(), MemoryEntry{Key: "rust-tips", Content: "Rust ownership rules"})
	mm.Store(context.Background(), MemoryEntry{Key: "go-testing", Content: "Go table-driven tests"})

	results, err := mm.Search(context.Background(), "go", 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) < 2 {
		t.Errorf("expected at least 2 results for 'go', got %d", len(results))
	}
}

func TestMemoryLocalDelete(t *testing.T) {
	tmpDir := t.TempDir()
	b, _ := NewBridge(BridgeConfig{
		GatewayURL:   "http://localhost:1",
		WorkspaceDir: tmpDir,
	})
	defer b.Close()
	mm := NewMemoryManager(b)

	mm.Store(context.Background(), MemoryEntry{Key: "temp-note", Content: "temporary"})
	err := mm.Delete(context.Background(), "temp-note")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err = mm.Retrieve(context.Background(), "temp-note")
	if err == nil {
		t.Error("expected error after delete")
	}
}

// SkillManager tests

func TestSkillLocalScan(t *testing.T) {
	// Create a temp skill directory
	home := t.TempDir()
	skillDir := filepath.Join(home, ".openclaw", "skills", "test-skill")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Test Skill\nA skill for testing purposes."), 0644)

	// Override home detection
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", home)
	defer os.Setenv("HOME", oldHome)

	b, _ := NewBridge(BridgeConfig{
		GatewayURL:   "http://localhost:1",
		WorkspaceDir: filepath.Join(home, "openclaw-workspace"),
	})
	defer b.Close()
	sm := NewSkillManager(b)

	skills, err := sm.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(skills) != 1 || skills[0].Name != "test-skill" {
		t.Errorf("expected 1 skill 'test-skill', got %v", skills)
	}
}

// BrowserController tests

func TestBrowserNavigate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/api/browser" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	b, _ := NewBridge(BridgeConfig{GatewayURL: srv.URL})
	defer b.Close()
	bc := NewBrowserController(b)

	err := bc.Navigate(context.Background(), BrowserRequest{URL: "https://example.com"})
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
}

func TestBrowserNavigateNoURL(t *testing.T) {
	b, _ := NewBridge(BridgeConfig{GatewayURL: "http://localhost:1"})
	defer b.Close()
	bc := NewBrowserController(b)

	err := bc.Navigate(context.Background(), BrowserRequest{})
	if err == nil {
		t.Error("expected error for empty URL")
	}
}

func TestBrowserSnapshot(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/api/browser" {
			json.NewEncoder(w).Encode(BrowserSnapshot{
				TargetID: "tab-1",
				Title:    "Example",
				URL:      "https://example.com",
				Content:  "Example Domain",
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	b, _ := NewBridge(BridgeConfig{GatewayURL: srv.URL})
	defer b.Close()
	bc := NewBrowserController(b)

	snap, err := bc.Snapshot(context.Background(), BrowserRequest{TargetID: "tab-1"})
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if snap.Title != "Example" {
		t.Errorf("expected Example, got %s", snap.Title)
	}
}
