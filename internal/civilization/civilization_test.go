package civilization

import (
	"path/filepath"
	"testing"
	"time"
)

func testIdentity() *OrgIdentity {
	return &OrgIdentity{
		ID:           "org-1",
		Name:         "TestOrg",
		Capabilities: []string{"code", "research"},
		FoundedAt:    time.Now().UTC(),
		Version:      ProtocolVersion,
	}
}

func TestRegisterPeer(t *testing.T) {
	hub := NewHub(testIdentity(), filepath.Join(t.TempDir(), "civ.json"))

	peer := &OrgIdentity{
		ID:           "org-2",
		Name:         "PeerOrg",
		Capabilities: []string{"data", "analysis"},
		FoundedAt:    time.Now().UTC(),
	}
	hub.RegisterPeer(peer)

	peers := hub.ListPeers()
	if len(peers) != 1 {
		t.Fatalf("expected 1 peer, got %d", len(peers))
	}
	if peers[0].Name != "PeerOrg" {
		t.Error("peer name mismatch")
	}
}

func TestMessaging(t *testing.T) {
	hub := NewHub(testIdentity(), filepath.Join(t.TempDir(), "civ.json"))

	msg, err := hub.SendMessage("org-2", "Collaboration", "Want to collaborate?", MsgProposal)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Version != ProtocolVersion {
		t.Error("protocol version mismatch")
	}
	if msg.FromOrg != "org-1" {
		t.Error("from org mismatch")
	}

	// Simulate incoming message
	incoming := &ProtocolMessage{
		Version:   ProtocolVersion,
		FromOrg:   "org-2",
		ToOrg:     "org-1",
		Type:      MsgAccept,
		Subject:   "Re: Collaboration",
		Body:      "Yes, let's collaborate!",
		Timestamp: time.Now().UTC(),
	}
	err = hub.ReceiveMessage(incoming)
	if err != nil {
		t.Fatal(err)
	}

	inbox := hub.GetInbox()
	if len(inbox) != 1 {
		t.Fatalf("expected 1 inbox message, got %d", len(inbox))
	}

	// Wrong recipient
	wrong := &ProtocolMessage{
		Version: ProtocolVersion,
		FromOrg: "org-3",
		ToOrg:   "org-999",
		Type:    MsgHello,
	}
	err = hub.ReceiveMessage(wrong)
	if err == nil {
		t.Error("should reject message for wrong org")
	}
}

func TestReputation(t *testing.T) {
	hub := NewHub(testIdentity(), filepath.Join(t.TempDir(), "civ.json"))

	hub.AddReputation("org-2", "org-1", "reliability", "Always delivers on time", 90, true)
	hub.AddReputation("org-3", "org-1", "quality", "High quality output", 85, true)
	hub.AddReputation("org-2", "org-1", "quality", "Good work", 75, false)

	score := hub.GetReputation("org-1")
	if score.EntryCount != 3 {
		t.Errorf("expected 3 entries, got %d", score.EntryCount)
	}
	if score.Overall <= 0 {
		t.Error("overall score should be positive")
	}
	if score.Categories["reliability"] != 90 {
		t.Errorf("reliability should be 90, got %f", score.Categories["reliability"])
	}
}

func TestMarketplace(t *testing.T) {
	hub := NewHub(testIdentity(), filepath.Join(t.TempDir(), "civ.json"))

	offering, err := hub.OfferService("Code Review", "AI-powered code review", "code", "per_call", "credits", 5)
	if err != nil {
		t.Fatal(err)
	}
	if !offering.Active {
		t.Error("offering should be active")
	}

	req, err := hub.RequestService("Data Analysis", "Need market analysis", "data", "credits", 100, nil)
	if err != nil {
		t.Fatal(err)
	}
	if req.Status != "open" {
		t.Error("request should be open")
	}

	tx, err := hub.RecordTransaction("org-2", "org-1", offering.ID, "credits", 50)
	if err != nil {
		t.Fatal(err)
	}
	hub.CompleteTransaction(tx.ID)
	tx, _ = hub.transactions[tx.ID]
	if tx.Status != "completed" {
		t.Error("transaction should be completed")
	}

	offerings := hub.ListOfferings("code")
	if len(offerings) != 1 {
		t.Errorf("expected 1 code offering, got %d", len(offerings))
	}
}

func TestFederation(t *testing.T) {
	hub := NewHub(testIdentity(), filepath.Join(t.TempDir(), "civ.json"))

	fed, err := hub.CreateFederation("Code Alliance", "AI code orgs unite", "Share best practices", []string{"org-2"})
	if err != nil {
		t.Fatal(err)
	}
	if len(fed.Members) != 2 { // org-2 + self
		t.Errorf("expected 2 initial members, got %d", len(fed.Members))
	}

	prop, err := hub.InviteToFederation(fed.ID, "org-3")
	if err != nil {
		t.Fatal(err)
	}
	if prop.Status != "pending" {
		t.Error("proposal should be pending")
	}

	hub.JoinFederation(fed.ID, "org-3")
	fed, _ = hub.federations[fed.ID]
	if len(fed.Members) != 3 {
		t.Errorf("expected 3 members after join, got %d", len(fed.Members))
	}

	feds := hub.ListFederations()
	if len(feds) != 1 {
		t.Error("org should belong to 1 federation")
	}
}

func TestTreaties(t *testing.T) {
	hub := NewHub(testIdentity(), filepath.Join(t.TempDir(), "civ.json"))

	treaty, err := hub.ProposeTreaty("Data Sharing Agreement", TreatyDataSharing,
		[]string{"org-1", "org-2"}, "Share anonymized usage data", nil)
	if err != nil {
		t.Fatal(err)
	}
	if treaty.Status != TreatyProposed {
		t.Error("treaty should be proposed")
	}

	hub.ActivateTreaty(treaty.ID)
	treaty, _ = hub.treaties[treaty.ID]
	if treaty.Status != TreatyActive {
		t.Error("treaty should be active")
	}

	viol, err := hub.ReportViolation(treaty.ID, "org-2", "Shared non-anonymized data", "logs show raw emails exported")
	if err != nil {
		t.Fatal(err)
	}
	if viol.Resolved {
		t.Error("violation should not be resolved")
	}

	treaty, _ = hub.treaties[treaty.ID]
	if treaty.Status != TreatyViolated {
		t.Error("treaty should be violated")
	}

	treaties := hub.ListTreaties(TreatyActive)
	if len(treaties) != 0 {
		t.Error("no active treaties after violation")
	}
}

func TestProcurement(t *testing.T) {
	hub := NewHub(testIdentity(), filepath.Join(t.TempDir(), "civ.json"))

	proc, err := hub.CreateProcurement("Cloud Credits", "AWS credits for compute", "cloud_credits", "usd", 1000, 500)
	if err != nil {
		t.Fatal(err)
	}
	if proc.Status != "draft" {
		t.Error("procurement should be draft")
	}

	hub.ApproveProcurement(proc.ID, "vendor-1")
	proc, _ = hub.procReqs[proc.ID]
	if proc.Status != "approved" {
		t.Error("should be approved")
	}

	hub.FulfillProcurement(proc.ID)
	proc, _ = hub.procReqs[proc.ID]
	if proc.Status != "fulfilled" {
		t.Error("should be fulfilled")
	}
}

func TestVendors(t *testing.T) {
	hub := NewHub(testIdentity(), filepath.Join(t.TempDir(), "civ.json"))

	endDate := time.Now().Add(365 * 24 * time.Hour)
	vendor, err := hub.AddVendor("AWS", "Cloud computing", "premium", &endDate)
	if err != nil {
		t.Fatal(err)
	}
	if vendor.Rating != 0 {
		t.Error("new vendor should have 0 rating")
	}

	hub.RateVendor(vendor.ID, 4.5)
	vendor, _ = hub.vendors[vendor.ID]
	if vendor.Rating != 4.5 {
		t.Errorf("expected rating 4.5, got %f", vendor.Rating)
	}

	err = hub.RateVendor(vendor.ID, 6)
	if err == nil {
		t.Error("should reject rating > 5")
	}

	vendors := hub.ListVendors()
	if len(vendors) != 1 {
		t.Error("should have 1 vendor")
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "civ.json")

	h1 := NewHub(testIdentity(), path)
	h1.RegisterPeer(&OrgIdentity{ID: "org-2", Name: "Peer", Capabilities: []string{"data"}})
	h1.OfferService("Test Service", "Desc", "code", "flat", "credits", 10)
	h1.ProposeTreaty("Test Treaty", TreatyTrade, []string{"org-1", "org-2"}, "terms", nil)

	h2 := NewHub(testIdentity(), path)
	if len(h2.peers) != 1 {
		t.Errorf("expected 1 loaded peer, got %d", len(h2.peers))
	}
	if len(h2.offerings) != 1 {
		t.Errorf("expected 1 loaded offering, got %d", len(h2.offerings))
	}
	if len(h2.treaties) != 1 {
		t.Errorf("expected 1 loaded treaty, got %d", len(h2.treaties))
	}
}
