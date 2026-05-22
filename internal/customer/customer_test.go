package customer

import (
	"path/filepath"
	"testing"
)

func TestRecordInteraction(t *testing.T) {
	cm := NewCustomerManager(filepath.Join(t.TempDir(), "customer.json"))
	i, err := cm.RecordInteraction("cust-1", ChannelChat, "pricing", "How much is Pro?", "Pro is $29/month", OriginAI, "bot-1")
	if err != nil {
		t.Fatal(err)
	}
	if i.Origin != OriginAI {
		t.Error("origin should be AI")
	}
	if i.Consistent != Unverified {
		t.Error("should be unverified without SSOT")
	}
}

func TestConsistencyWithSSOT(t *testing.T) {
	cm := NewCustomerManager(filepath.Join(t.TempDir(), "customer.json"))
	cm.SetAuthoritativeAnswer("pricing", "Pro is $29 per month billed annually", "product", "cfo")

	// Consistent answer
	i1, _ := cm.RecordInteraction("cust-1", ChannelChat, "pricing", "Price?", "Pro is $29 per month billed annually", OriginAI, "bot-1")
	if i1.Consistent != Consistent {
		t.Errorf("expected consistent, got %s", i1.Consistent)
	}

	// Different answer
	i2, _ := cm.RecordInteraction("cust-2", ChannelEmail, "pricing", "Price?", "Pro costs $49 per month", OriginHuman, "agent-1")
	if i2.Consistent != Inconsistent {
		t.Errorf("expected inconsistent, got %s", i2.Consistent)
	}
}

func TestCheckConsistency(t *testing.T) {
	cm := NewCustomerManager(filepath.Join(t.TempDir(), "customer.json"))
	cm.SetAuthoritativeAnswer("refund", "Refunds are processed within 5 business days", "legal", "legal-team")

	i, _ := cm.RecordInteraction("cust-1", ChannelChat, "refund", "Refund timeline?", "Refunds take about 5 business days", OriginAI, "bot-1")

	check, err := cm.CheckConsistency(i.ID)
	if err != nil {
		t.Fatal(err)
	}
	if check.Status == "" {
		t.Error("status should be set")
	}
	if check.Similarity <= 0 {
		t.Error("similarity should be positive")
	}
}

func TestCheckConsistencyNoSSOT(t *testing.T) {
	cm := NewCustomerManager(filepath.Join(t.TempDir(), "customer.json"))
	i, _ := cm.RecordInteraction("cust-1", ChannelChat, "unknown-topic", "Q?", "A", OriginAI, "bot-1")

	check, err := cm.CheckConsistency(i.ID)
	if err != nil {
		t.Fatal(err)
	}
	if check.Status != Unverified {
		t.Error("should be unverified without SSOT")
	}
}

func TestGetAuthoritativeAnswer(t *testing.T) {
	cm := NewCustomerManager(filepath.Join(t.TempDir(), "customer.json"))
	cm.SetAuthoritativeAnswer("pricing", "Pro is $29/month", "product", "cfo")

	ssot, err := cm.GetAuthoritativeAnswer("pricing")
	if err != nil {
		t.Fatal(err)
	}
	if ssot.Answer != "Pro is $29/month" {
		t.Error("answer mismatch")
	}

	_, err = cm.GetAuthoritativeAnswer("nonexistent")
	if err == nil {
		t.Error("should error on missing topic")
	}
}

func TestUpdateSSOT(t *testing.T) {
	cm := NewCustomerManager(filepath.Join(t.TempDir(), "customer.json"))
	s1, _ := cm.SetAuthoritativeAnswer("pricing", "$29/month", "product", "cfo")
	s2, _ := cm.SetAuthoritativeAnswer("pricing", "$39/month", "product", "cfo")
	if s2.Version != 2 {
		t.Errorf("expected version 2, got %d", s2.Version)
	}
	if s1.ID != s2.ID {
		t.Error("updating same topic should reuse ID")
	}
}

func TestResponsePolicy(t *testing.T) {
	cm := NewCustomerManager(filepath.Join(t.TempDir(), "customer.json"))
	cm.SetResponsePolicy("Chat Disclosure", ChannelChat, true, true, "This response was AI-generated")

	pol, err := cm.GetResponsePolicy(ChannelChat)
	if err != nil {
		t.Fatal(err)
	}
	if !pol.RevealAI {
		t.Error("should reveal AI")
	}
	if pol.SignaturePhrase != "This response was AI-generated" {
		t.Error("phrase mismatch")
	}

	_, err = cm.GetResponsePolicy(ChannelPhone)
	if err == nil {
		t.Error("should error on missing policy")
	}
}

func TestTextSimilarity(t *testing.T) {
	tests := []struct {
		a, b     string
		min, max float64
	}{
		{"hello world", "hello world", 0.9, 1.0},
		{"the cat sat", "the dog sat", 0.3, 0.8},
		{"completely different", "totally unrelated stuff", 0.0, 0.3},
		{"", "", 1.0, 1.0},
		{"word", "", 0.0, 0.0},
	}
	for _, tt := range tests {
		sim := textSimilarity(tt.a, tt.b)
		if sim < tt.min || sim > tt.max {
			t.Errorf("similarity(%q, %q) = %.2f, expected [%.2f, %.2f]", tt.a, tt.b, sim, tt.min, tt.max)
		}
	}
}

func TestListInteractions(t *testing.T) {
	cm := NewCustomerManager(filepath.Join(t.TempDir(), "customer.json"))
	cm.RecordInteraction("cust-1", ChannelChat, "pricing", "Q1", "A1", OriginAI, "bot-1")
	cm.RecordInteraction("cust-1", ChannelEmail, "billing", "Q2", "A2", OriginHuman, "agent-1")
	cm.RecordInteraction("cust-2", ChannelChat, "pricing", "Q3", "A3", OriginAI, "bot-1")

	list := cm.ListInteractions("cust-1")
	if len(list) != 2 {
		t.Errorf("expected 2 interactions for cust-1, got %d", len(list))
	}

	list2 := cm.ListInteractions("cust-2")
	if len(list2) != 1 {
		t.Errorf("expected 1 interaction for cust-2, got %d", len(list2))
	}
}

func TestGenerateTransparencyReport(t *testing.T) {
	cm := NewCustomerManager(filepath.Join(t.TempDir(), "customer.json"))
	cm.SetAuthoritativeAnswer("pricing", "Pro is $29/month", "product", "cfo")
	cm.RecordInteraction("cust-1", ChannelChat, "pricing", "Q?", "Pro is $29/month", OriginAI, "bot-1")
	cm.RecordInteraction("cust-2", ChannelEmail, "pricing", "Q?", "$49/month", OriginHuman, "agent-1")

	report := cm.GenerateTransparencyReport()
	if report["total_interactions"].(int) != 2 {
		t.Errorf("expected 2 interactions, got %v", report["total_interactions"])
	}
	if report["ai_responses"].(int) != 1 {
		t.Errorf("expected 1 AI response, got %v", report["ai_responses"])
	}
	if report["ssot_entries"].(int) != 1 {
		t.Errorf("expected 1 SSOT, got %v", report["ssot_entries"])
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "customer.json")

	cm1 := NewCustomerManager(path)
	cm1.SetAuthoritativeAnswer("pricing", "$29/month", "product", "cfo")
	cm1.RecordInteraction("cust-1", ChannelChat, "pricing", "Q?", "$29/month", OriginAI, "bot-1")

	cm2 := NewCustomerManager(path)
	if len(cm2.interactions) != 1 {
		t.Errorf("expected 1 interaction, got %d", len(cm2.interactions))
	}
	if len(cm2.ssots) != 1 {
		t.Errorf("expected 1 SSOT, got %d", len(cm2.ssots))
	}
}
