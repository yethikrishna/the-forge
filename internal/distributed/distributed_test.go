package distributed

import (
	"path/filepath"
	"testing"
	"time"
)

func tempStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s := NewStore(filepath.Join(dir, "distributed.json"), DefaultConfig())
	if err := s.Load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	return s
}

func TestRegisterNode(t *testing.T) {
	s := tempStore(t)
	node := s.RegisterNode(FederationNode{
		OrgID:    "org1",
		Endpoint: "https://node1.example.com",
		Region:   "us-east-1",
	})
	if node.ID == "" {
		t.Fatal("expected auto-generated ID")
	}
	if node.Status != "active" {
		t.Fatalf("expected default status active, got %s", node.Status)
	}
}

func TestPerformBackup(t *testing.T) {
	s := tempStore(t)
	node := s.RegisterNode(FederationNode{OrgID: "org1", Region: "us-east"})
	backup := s.PerformBackup("org1", node.ID, 1024000, "abc123")
	if backup.ID == "" {
		t.Fatal("expected auto-generated backup ID")
	}
	if backup.Status != "completed" {
		t.Fatalf("expected completed status, got %s", backup.Status)
	}
	if backup.OrgID != "org1" {
		t.Fatalf("expected org1, got %s", backup.OrgID)
	}
}

func TestCheckResilience_NoNodes(t *testing.T) {
	s := tempStore(t)
	rs := s.CheckResilience("ghost-org")
	if rs.Score != 0 {
		t.Fatalf("expected 0 score for org with no nodes, got %f", rs.Score)
	}
	if rs.CanRestore {
		t.Fatal("expected CanRestore=false with no nodes")
	}
}

func TestCheckResilience_FullSetup(t *testing.T) {
	s := tempStore(t)
	for i := 0; i < 3; i++ {
		s.RegisterNode(FederationNode{
			OrgID:    "org1",
			Endpoint: "https://node.example.com",
			Region:   "us-east",
			Status:   "active",
		})
	}
	s.PerformBackup("org1", "node-1", 5000, "sha256")

	rs := s.CheckResilience("org1")
	if rs.Score < 0.9 {
		t.Fatalf("expected high resilience, got %f", rs.Score)
	}
	if rs.ActiveNodes != 3 {
		t.Fatalf("expected 3 active nodes, got %d", rs.ActiveNodes)
	}
	if !rs.CanRestore {
		t.Fatal("expected CanRestore=true")
	}
}

func TestRestoreFromBackup(t *testing.T) {
	s := tempStore(t)
	s.PerformBackup("org1", "n1", 1000, "ck1")
	time.Sleep(time.Millisecond)
	s.PerformBackup("org1", "n2", 2000, "ck2")

	backup, ok := s.RestoreFromBackup("org1")
	if !ok {
		t.Fatal("expected to find a backup")
	}
	if backup.Checksum != "ck2" {
		t.Fatalf("expected latest backup ck2, got %s", backup.Checksum)
	}
}

func TestRestoreFromBackup_NoBackup(t *testing.T) {
	s := tempStore(t)
	_, ok := s.RestoreFromBackup("no-backup-org")
	if ok {
		t.Fatal("expected no backup found")
	}
}

func TestGenerateFederationReport(t *testing.T) {
	s := tempStore(t)
	s.RegisterNode(FederationNode{OrgID: "org1", Region: "us", Status: "active"})
	s.RegisterNode(FederationNode{OrgID: "org1", Region: "eu", Status: "standby"})
	s.PerformBackup("org1", "n1", 100, "x")

	report := s.GenerateFederationReport()
	if report["total_nodes"] != 2 {
		t.Fatalf("expected 2 nodes, got %v", report["total_nodes"])
	}
}

func TestStorePersistence(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "dist.json")

	s1 := NewStore(fp, DefaultConfig())
	s1.RegisterNode(FederationNode{OrgID: "org1", Region: "us"})
	if err := s1.Save(); err != nil {
		t.Fatal(err)
	}

	s2 := NewStore(fp, DefaultConfig())
	if err := s2.Load(); err != nil {
		t.Fatal(err)
	}
	report := s2.GenerateFederationReport()
	if report["total_nodes"] != 1 {
		t.Fatalf("expected 1 node after load, got %v", report["total_nodes"])
	}
}
