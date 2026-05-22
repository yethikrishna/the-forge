package dashboard

import (
	"testing"

	"github.com/forge/sword/internal/org"
	"github.com/forge/sword/internal/trust"
)

func TestLiveProviderReturnsRealOrgData(t *testing.T) {
	dir := t.TempDir()

	// Create a real org with bootstrap
	o := org.New("TestCorp", "human", dir+"/org.json")
	result, err := o.Bootstrap()
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if len(result.Divisions) != 4 {
		t.Fatalf("expected 4 divisions, got %d", len(result.Divisions))
	}

	tm := trust.NewManager(dir + "/trust")

	hub := NewWebSocketHub()
	go hub.Run()

	p := NewLiveProvider(LiveProviderConfig{
		Org:      o,
		TrustMgr: tm,
		Hub:      hub,
	})

	// Stats should show real agent count (4 head agents from bootstrap)
	stats := p.GetStats()
	if stats.ActiveAgents != 4 {
		t.Errorf("expected 4 active agents, got %d", stats.ActiveAgents)
	}

	// GetAgents should return real agents
	agents := p.GetAgents()
	if len(agents) != 4 {
		t.Errorf("expected 4 agents, got %d", len(agents))
	}

	// Verify agent names match bootstrapped head agents
	namesSeen := map[string]bool{}
	for _, a := range agents {
		namesSeen[a.Name] = true
	}
	for _, expected := range []string{"Arch-1", "Research-Lead-1", "Ops-Lead-1", "SecLead-1"} {
		if !namesSeen[expected] {
			t.Errorf("expected agent %q not found in dashboard", expected)
		}
	}

	t.Logf("Dashboard shows %d real agents from org", len(agents))
	for _, a := range agents {
		t.Logf("  Agent: %s (%s) status=%s", a.Name, a.Type, a.Status)
	}
}

func TestLiveProviderWebSocketBroadcast(t *testing.T) {
	dir := t.TempDir()
	o := org.New("BroadcastCorp", "human", dir+"/org.json")
	if _, err := o.Bootstrap(); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	hub := NewWebSocketHub()
	go hub.Run()

	p := NewLiveProvider(LiveProviderConfig{
		Org: o,
		Hub: hub,
	})

	// AddLog should not panic even with no WS clients
	p.AddLog("info", "test log entry")
	log := p.GetLog()
	if len(log) != 1 {
		t.Errorf("expected 1 log entry, got %d", len(log))
	}
	if log[0].Message != "test log entry" {
		t.Errorf("unexpected log message: %s", log[0].Message)
	}

	// PushStateUpdate with no clients should not block or panic
	p.PushStateUpdate()
}
