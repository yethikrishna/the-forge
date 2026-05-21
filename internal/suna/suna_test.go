package suna

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// Bridge tests

func TestNewBridge(t *testing.T) {
	b, err := NewBridge(BridgeConfig{})
	if err != nil {
		t.Fatalf("NewBridge: %v", err)
	}
	defer b.Close()
	if b.FrontendURL() != "http://localhost:3000" {
		t.Errorf("expected default frontend URL, got %s", b.FrontendURL())
	}
	if b.APIURL() != "http://localhost:8000" {
		t.Errorf("expected default API URL, got %s", b.APIURL())
	}
}

func TestBridgeHealth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	b, _ := NewBridge(BridgeConfig{APIURL: srv.URL})
	defer b.Close()

	if err := b.Health(context.Background()); err != nil {
		t.Fatalf("health check: %v", err)
	}
}

func TestBridgeAuth(t *testing.T) {
	var apiKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey = r.Header.Get("X-API-Key")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	b, _ := NewBridge(BridgeConfig{APIURL: srv.URL, APIKey: "test-key"})
	defer b.Close()

	b.GetJSON(context.Background(), "/api/test", nil)
	if apiKey != "test-key" {
		t.Errorf("expected test-key, got %s", apiKey)
	}
}

func TestBridgeClosed(t *testing.T) {
	b, _ := NewBridge(BridgeConfig{APIURL: "http://localhost:1"})
	b.Close()
	if err := b.Health(context.Background()); err == nil {
		t.Error("expected error on closed bridge")
	}
}

// Sandbox tests

func TestSandboxCreateValidation(t *testing.T) {
	b, _ := NewBridge(BridgeConfig{})
	defer b.Close()
	sm := NewSandboxManager(b)

	_, err := sm.Create(context.Background(), SandboxConfig{})
	if err == nil {
		t.Error("expected error for empty name")
	}

	_, err = sm.Create(context.Background(), SandboxConfig{Name: "test"})
	if err == nil {
		t.Error("expected error for empty agent_id")
	}
}

func TestSandboxCreate(t *testing.T) {
	if os.Getenv("DOCKER_HOST") == "" && !dockerAvailable() {
		t.Skip("Docker not available")
	}

	b, _ := NewBridge(BridgeConfig{SandboxDockerImage: "alpine:latest"})
	defer b.Close()
	sm := NewSandboxManager(b)

	sb, err := sm.Create(context.Background(), SandboxConfig{
		Name:    "test-sb",
		AgentID: "agent-1",
		Image:   "alpine:latest",
		Resources: SandboxResources{
			CPUCores:    1.0,
			MemoryMB:    256,
			DiskMB:      1024,
			TimeoutSecs: 60,
		},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer sm.Remove(context.Background(), "test-sb")

	if sb.State != SandboxRunning {
		t.Errorf("expected running, got %s", sb.State)
	}
	if sb.ContainerID == "" {
		t.Error("expected container ID")
	}
}

func TestSandboxExec(t *testing.T) {
	if os.Getenv("DOCKER_HOST") == "" && !dockerAvailable() {
		t.Skip("Docker not available")
	}

	b, _ := NewBridge(BridgeConfig{SandboxDockerImage: "alpine:latest"})
	defer b.Close()
	sm := NewSandboxManager(b)

	_, err := sm.Create(context.Background(), SandboxConfig{
		Name:    "exec-test",
		AgentID: "agent-1",
		Image:   "alpine:latest",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer sm.Remove(context.Background(), "exec-test")

	// Alpine exits immediately with echo, need a long-running command
	// Recreate with a sleep so it stays up
	sm.Remove(context.Background(), "exec-test")
	_, err = sm.Create(context.Background(), SandboxConfig{
		Name:    "exec-test",
		AgentID: "agent-1",
		Image:   "alpine:latest",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer sm.Remove(context.Background(), "exec-test")

	out, err := sm.Exec(context.Background(), "exec-test", "echo hello")
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if out != "hello\n" {
		t.Errorf("expected 'hello\\n', got %q", out)
	}
}

func TestSandboxList(t *testing.T) {
	b, _ := NewBridge(BridgeConfig{})
	defer b.Close()
	sm := NewSandboxManager(b)

	// Add fake sandboxes
	sm.mu.Lock()
	sm.sboxes["sb1"] = &Sandbox{Name: "sb1", AgentID: "a1", Division: "eng", State: SandboxRunning}
	sm.sboxes["sb2"] = &Sandbox{Name: "sb2", AgentID: "a2", Division: "ops", State: SandboxRunning}
	sm.sboxes["sb3"] = &Sandbox{Name: "sb3", AgentID: "a1", Division: "eng", State: SandboxStopped}
	sm.mu.Unlock()

	all := sm.List("", "")
	if len(all) != 3 {
		t.Errorf("expected 3, got %d", len(all))
	}

	byAgent := sm.List("a1", "")
	if len(byAgent) != 2 {
		t.Errorf("expected 2 for agent a1, got %d", len(byAgent))
	}

	byDiv := sm.List("", "eng")
	if len(byDiv) != 2 {
		t.Errorf("expected 2 for division eng, got %d", len(byDiv))
	}
}

// Skills tests

func TestSkillList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/skills" {
			json.NewEncoder(w).Encode([]*Skill{
				{ID: "sk1", Name: "Web Search", Category: CategoryResearch},
				{ID: "sk2", Name: "Data Analysis", Category: CategoryDataAnalysis},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	b, _ := NewBridge(BridgeConfig{APIURL: srv.URL})
	defer b.Close()
	sm := NewSkillManager(b)

	skills, err := sm.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(skills) != 2 {
		t.Errorf("expected 2 skills, got %d", len(skills))
	}
}

func TestSkillForDivision(t *testing.T) {
	b, _ := NewBridge(BridgeConfig{})
	defer b.Close()
	sm := NewSkillManager(b)

	cats := sm.ForDivision("engineering")
	if len(cats) == 0 {
		t.Error("expected categories for engineering")
	}

	cats = sm.ForDivision("nonexistent")
	if len(cats) == 0 {
		t.Error("expected default categories for unknown division")
	}
}

// Integrations tests

func TestIntegrationList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/integrations" {
			json.NewEncoder(w).Encode([]*Integration{
				{ID: "int1", Name: "GitHub", Provider: "github", Status: IntConnected},
				{ID: "int2", Name: "Slack", Provider: "slack", Status: IntConnected},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	b, _ := NewBridge(BridgeConfig{APIURL: srv.URL})
	defer b.Close()
	im := NewIntegrationManager(b)

	integrations, err := im.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(integrations) != 2 {
		t.Errorf("expected 2, got %d", len(integrations))
	}
}

func TestIntegrationIsConnected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/integrations" {
			json.NewEncoder(w).Encode([]*Integration{
				{ID: "int1", Provider: "github", Status: IntConnected},
				{ID: "int2", Provider: "slack", Status: IntDisconnected},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	b, _ := NewBridge(BridgeConfig{APIURL: srv.URL})
	defer b.Close()
	im := NewIntegrationManager(b)

	if !im.IsConnected(context.Background(), "github") {
		t.Error("expected github to be connected")
	}
	if im.IsConnected(context.Background(), "slack") {
		t.Error("expected slack to be disconnected")
	}
	if im.IsConnected(context.Background(), "nonexistent") {
		t.Error("expected nonexistent to be disconnected")
	}
}

// Marketplace tests

func TestMarketplaceBrowse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/marketplace" {
			json.NewEncoder(w).Encode([]*MarketplaceSkill{
				{ID: "ms1", Name: "Super Search", Downloads: 500},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	b, _ := NewBridge(BridgeConfig{APIURL: srv.URL})
	defer b.Close()
	m := NewMarketplace(b)

	skills, err := m.Browse(context.Background(), BrowseOpts{Limit: 10})
	if err != nil {
		t.Fatalf("Browse: %v", err)
	}
	if len(skills) != 1 {
		t.Errorf("expected 1, got %d", len(skills))
	}
}

func TestMarketplacePublishValidation(t *testing.T) {
	b, _ := NewBridge(BridgeConfig{})
	defer b.Close()
	m := NewMarketplace(b)

	_, err := m.Publish(context.Background(), PublishRequest{})
	if err == nil {
		t.Error("expected error for empty name")
	}

	_, err = m.Publish(context.Background(), PublishRequest{Name: "test"})
	if err == nil {
		t.Error("expected error for empty description")
	}
}

// Mobile tests

func TestMobileNotification(t *testing.T) {
	var received map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/api/mobile/notify" {
			json.NewDecoder(r.Body).Decode(&received)
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	b, _ := NewBridge(BridgeConfig{APIURL: srv.URL})
	defer b.Close()
	mm := NewMobileManager(b)

	err := mm.SendNotification(context.Background(), MobileNotification{
		Type:    NotifAlert,
		Title:   "Build Failed",
		Body:    "Engineering division build #42 failed",
		Priority: 3,
	})
	if err != nil {
		t.Fatalf("SendNotification: %v", err)
	}
	if received["title"] != "Build Failed" {
		t.Errorf("expected Build Failed, got %v", received["title"])
	}
}

func TestMobileNotificationValidation(t *testing.T) {
	b, _ := NewBridge(BridgeConfig{})
	defer b.Close()
	mm := NewMobileManager(b)

	err := mm.SendNotification(context.Background(), MobileNotification{})
	if err == nil {
		t.Error("expected error for empty title")
	}
}

func TestMobileApproval(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/api/mobile/approval" {
			json.NewEncoder(w).Encode(map[string]string{"id": "appr-123"})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	b, _ := NewBridge(BridgeConfig{APIURL: srv.URL})
	defer b.Close()
	mm := NewMobileManager(b)

	id, err := mm.RequestApproval(context.Background(), MobileApproval{
		Title:     "Deploy to Production",
		RiskLevel: "high",
	})
	if err != nil {
		t.Fatalf("RequestApproval: %v", err)
	}
	if id != "appr-123" {
		t.Errorf("expected appr-123, got %s", id)
	}
}

// Trigger tests

func TestTriggerCreate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/api/triggers" {
			json.NewEncoder(w).Encode(map[string]string{"id": "trig-123"})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	b, _ := NewBridge(BridgeConfig{APIURL: srv.URL})
	defer b.Close()
	tm := NewTriggerManager(b)

	trigger, err := tm.Create(context.Background(), TriggerConfig{
		Name:     "daily-standup",
		Type:     TriggerCron,
		Schedule: "0 9 * * *",
		Task:     "run-standup",
		Division: "engineering",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if trigger.State != TriggerActive {
		t.Errorf("expected active, got %s", trigger.State)
	}
}

func TestTriggerCreateValidation(t *testing.T) {
	b, _ := NewBridge(BridgeConfig{})
	defer b.Close()
	tm := NewTriggerManager(b)

	_, err := tm.Create(context.Background(), TriggerConfig{})
	if err == nil {
		t.Error("expected error for empty name")
	}

	_, err = tm.Create(context.Background(), TriggerConfig{Name: "test", Task: "do"})
	if err == nil {
		t.Error("expected error when API is unreachable")
	}

	_, err = tm.Create(context.Background(), TriggerConfig{Name: "test", Type: TriggerCron, Task: "do"})
	if err == nil {
		t.Error("expected error for cron without schedule")
	}
}

func TestTriggerFire(t *testing.T) {
	b, _ := NewBridge(BridgeConfig{})
	defer b.Close()
	tm := NewTriggerManager(b)

	// Create trigger locally
	tm.mu.Lock()
	tm.triggers["trig-1"] = &Trigger{
		ID:   "trig-1",
		Name: "test-trigger",
		Task: "echo hello",
	}
	tm.mu.Unlock()

	// Register handler
	called := false
	tm.RegisterHandler("trig-1", func(ctx context.Context, event TriggerEvent) error {
		called = true
		return nil
	})

	err := tm.Fire(context.Background(), TriggerEvent{TriggerID: "trig-1"})
	if err != nil {
		t.Fatalf("Fire: %v", err)
	}
	if !called {
		t.Error("handler was not called")
	}

	// Check fire count
	trigger, _ := tm.Get("trig-1")
	if trigger.FireCount != 1 {
		t.Errorf("expected 1 fire count, got %d", trigger.FireCount)
	}
}

func TestTriggerFireNotFound(t *testing.T) {
	b, _ := NewBridge(BridgeConfig{})
	defer b.Close()
	tm := NewTriggerManager(b)

	err := tm.Fire(context.Background(), TriggerEvent{TriggerID: "nonexistent"})
	if err == nil {
		t.Error("expected error for nonexistent trigger")
	}
}

func TestTriggerList(t *testing.T) {
	b, _ := NewBridge(BridgeConfig{})
	defer b.Close()
	tm := NewTriggerManager(b)

	tm.mu.Lock()
	tm.triggers["t1"] = &Trigger{ID: "t1", Type: TriggerCron, Division: "eng"}
	tm.triggers["t2"] = &Trigger{ID: "t2", Type: TriggerWebhook, Division: "ops"}
	tm.triggers["t3"] = &Trigger{ID: "t3", Type: TriggerCron, Division: "eng"}
	tm.mu.Unlock()

	all := tm.List("", "")
	if len(all) != 3 {
		t.Errorf("expected 3, got %d", len(all))
	}

	cronTriggers := tm.List(TriggerCron, "")
	if len(cronTriggers) != 2 {
		t.Errorf("expected 2 cron triggers, got %d", len(cronTriggers))
	}

	engTriggers := tm.List("", "eng")
	if len(engTriggers) != 2 {
		t.Errorf("expected 2 eng triggers, got %d", len(engTriggers))
	}
}

func TestTriggerPauseResume(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	b, _ := NewBridge(BridgeConfig{APIURL: srv.URL})
	defer b.Close()
	tm := NewTriggerManager(b)

	tm.mu.Lock()
	tm.triggers["t1"] = &Trigger{ID: "t1", State: TriggerActive}
	tm.mu.Unlock()

	if err := tm.Pause(context.Background(), "t1"); err != nil {
		t.Fatalf("Pause: %v", err)
	}
	trig, _ := tm.Get("t1")
	if trig.State != TriggerPaused {
		t.Errorf("expected paused, got %s", trig.State)
	}

	if err := tm.Resume(context.Background(), "t1"); err != nil {
		t.Fatalf("Resume: %v", err)
	}
	trig, _ = tm.Get("t1")
	if trig.State != TriggerActive {
		t.Errorf("expected active, got %s", trig.State)
	}
}

func dockerAvailable() bool {
	_, err := os.Stat("/var/run/docker.sock")
	return err == nil
}
