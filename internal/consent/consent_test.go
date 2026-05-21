package consent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func tempConsentStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return s
}

func TestGrant(t *testing.T) {
	s := tempConsentStore(t)

	r, err := s.Grant("user-1", "tenant-1",
		[]Purpose{PurposeAgentExecution, PurposeMemory},
		[]DataCategory{DataSourceCode, DataConversations},
		WithDescription("Full agent access"),
		WithSource("cli"),
	)
	if err != nil {
		t.Fatalf("Grant: %v", err)
	}
	if r.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if r.Status != StatusGranted {
		t.Fatalf("expected granted, got %s", r.Status)
	}
	if r.UserID != "user-1" {
		t.Fatal("wrong user")
	}
	if len(r.Purposes) != 2 {
		t.Fatalf("expected 2 purposes, got %d", len(r.Purposes))
	}
	if r.Checksum == "" {
		t.Fatal("expected checksum")
	}
	if r.Source != "cli" {
		t.Fatalf("expected cli source, got %s", r.Source)
	}
}

func TestGrantWithExpiry(t *testing.T) {
	s := tempConsentStore(t)

	r, err := s.Grant("user-1", "",
		[]Purpose{PurposeAnalytics},
		[]DataCategory{DataMetrics},
		WithExpiry(24*time.Hour),
	)
	if err != nil {
		t.Fatalf("Grant: %v", err)
	}
	if r.ExpiresAt == nil {
		t.Fatal("expected expiry")
	}
	if r.ExpiresAt.Before(r.GrantedAt) {
		t.Fatal("expiry should be after grant")
	}
}

func TestRevoke(t *testing.T) {
	s := tempConsentStore(t)

	r, _ := s.Grant("user-1", "",
		[]Purpose{PurposeAgentExecution},
		[]DataCategory{DataSourceCode},
	)

	revoked, err := s.Revoke(r.ID, "no longer needed")
	if err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if revoked.Status != StatusRevoked {
		t.Fatalf("expected revoked, got %s", revoked.Status)
	}
	if revoked.RevokedAt == nil {
		t.Fatal("expected revoked_at")
	}
	if revoked.WithdrawalReason != "no longer needed" {
		t.Fatal("wrong reason")
	}
}

func TestWithdraw(t *testing.T) {
	s := tempConsentStore(t)

	r, _ := s.Grant("user-1", "",
		[]Purpose{PurposeTelemetry},
		[]DataCategory{DataMetrics},
	)

	withdrawn, err := s.Withdraw(r.ID, "user-1", "GDPR withdrawal")
	if err != nil {
		t.Fatalf("Withdraw: %v", err)
	}
	if withdrawn.Status != StatusWithdrawn {
		t.Fatalf("expected withdrawn, got %s", withdrawn.Status)
	}
}

func TestWithdrawWrongUser(t *testing.T) {
	s := tempConsentStore(t)

	r, _ := s.Grant("user-1", "",
		[]Purpose{PurposeTelemetry},
		[]DataCategory{DataMetrics},
	)

	_, err := s.Withdraw(r.ID, "user-2", "wrong user")
	if err == nil {
		t.Fatal("expected error for wrong user")
	}
}

func TestCheck(t *testing.T) {
	s := tempConsentStore(t)

	// No consent yet.
	ok, _, _ := s.Check("user-1", PurposeAgentExecution)
	if ok {
		t.Fatal("should not have consent")
	}

	// Grant consent.
	s.Grant("user-1", "",
		[]Purpose{PurposeAgentExecution},
		[]DataCategory{DataSourceCode},
	)

	ok, r, _ := s.Check("user-1", PurposeAgentExecution)
	if !ok {
		t.Fatal("should have consent")
	}
	if r == nil {
		t.Fatal("expected record")
	}

	// Different purpose.
	ok, _, _ = s.Check("user-1", PurposeTraining)
	if ok {
		t.Fatal("should not have consent for training")
	}
}

func TestCheckExpired(t *testing.T) {
	s := tempConsentStore(t)

	s.Grant("user-1", "",
		[]Purpose{PurposeAnalytics},
		[]DataCategory{DataMetrics},
		WithExpiry(-1*time.Hour), // Already expired
	)

	ok, _, _ := s.Check("user-1", PurposeAnalytics)
	if ok {
		t.Fatal("expired consent should not pass check")
	}
}

func TestExpire(t *testing.T) {
	s := tempConsentStore(t)

	s.Grant("user-1", "",
		[]Purpose{PurposeAnalytics},
		[]DataCategory{DataMetrics},
		WithExpiry(-1*time.Hour),
	)
	s.Grant("user-2", "",
		[]Purpose{PurposeAgentExecution},
		[]DataCategory{DataSourceCode},
		WithExpiry(24*time.Hour), // Still valid
	)

	count, err := s.Expire()
	if err != nil {
		t.Fatalf("Expire: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 expired, got %d", count)
	}

	// Verify the expired record.
	r, _ := s.Get(func() string {
		for _, r := range s.records {
			if r.UserID == "user-1" {
				return r.ID
			}
		}
		return ""
	}())
	if r.Status != StatusExpired {
		t.Fatalf("expected expired, got %s", r.Status)
	}
}

func TestList(t *testing.T) {
	s := tempConsentStore(t)

	s.Grant("user-1", "", []Purpose{PurposeAgentExecution}, []DataCategory{DataSourceCode})
	s.Grant("user-2", "", []Purpose{PurposeAnalytics}, []DataCategory{DataMetrics})
	s.Grant("user-1", "", []Purpose{PurposeMemory}, []DataCategory{DataConversations})

	// All records.
	all, _ := s.List(map[string]string{})
	if len(all) != 3 {
		t.Fatalf("expected 3 records, got %d", len(all))
	}

	// Filter by user.
	u1, _ := s.List(map[string]string{"user_id": "user-1"})
	if len(u1) != 2 {
		t.Fatalf("expected 2 records for user-1, got %d", len(u1))
	}

	// Filter by purpose.
	analytics, _ := s.List(map[string]string{"purpose": "analytics"})
	if len(analytics) != 1 {
		t.Fatalf("expected 1 analytics record, got %d", len(analytics))
	}
}

func TestListByUser(t *testing.T) {
	s := tempConsentStore(t)

	s.Grant("user-1", "", []Purpose{PurposeAgentExecution}, []DataCategory{DataSourceCode})
	s.Grant("user-2", "", []Purpose{PurposeAnalytics}, []DataCategory{DataMetrics})

	records, _ := s.ListByUser("user-1")
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
}

func TestVerifyIntegrity(t *testing.T) {
	s := tempConsentStore(t)

	s.Grant("user-1", "", []Purpose{PurposeAgentExecution}, []DataCategory{DataSourceCode})
	s.Grant("user-1", "", []Purpose{PurposeMemory}, []DataCategory{DataConversations})

	ok, issues, err := s.Verify()
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !ok {
		t.Fatalf("integrity check failed: %v", issues)
	}
}

func TestGetStats(t *testing.T) {
	s := tempConsentStore(t)

	s.Grant("user-1", "", []Purpose{PurposeAgentExecution}, []DataCategory{DataSourceCode})
	s.Grant("user-2", "", []Purpose{PurposeAnalytics}, []DataCategory{DataMetrics})
	r3, _ := s.Grant("user-3", "", []Purpose{PurposeTelemetry}, []DataCategory{DataMetrics})
	s.Revoke(r3.ID, "testing")

	stats := s.GetStats()
	if stats.TotalRecords != 3 {
		t.Fatalf("expected 3 records, got %d", stats.TotalRecords)
	}
	if stats.GrantedCount != 2 {
		t.Fatalf("expected 2 granted, got %d", stats.GrantedCount)
	}
	if stats.RevokedCount != 1 {
		t.Fatalf("expected 1 revoked, got %d", stats.RevokedCount)
	}
	if stats.AuditTrailCount < 4 {
		t.Fatalf("expected at least 4 audit entries, got %d", stats.AuditTrailCount)
	}
}

func TestAuditTrail(t *testing.T) {
	s := tempConsentStore(t)

	r, _ := s.Grant("user-1", "", []Purpose{PurposeAgentExecution}, []DataCategory{DataSourceCode})
	s.Revoke(r.ID, "done")

	trail, err := s.GetAuditTrail(r.ID)
	if err != nil {
		t.Fatalf("GetAuditTrail: %v", err)
	}
	if len(trail) != 2 {
		t.Fatalf("expected 2 audit entries, got %d", len(trail))
	}
	if trail[0].Action != "grant" {
		t.Fatalf("expected grant first, got %s", trail[0].Action)
	}
	if trail[1].Action != "revoke" {
		t.Fatalf("expected revoke second, got %s", trail[1].Action)
	}
}

func TestPolicies(t *testing.T) {
	s := tempConsentStore(t)

	p, err := s.CreatePolicy(Policy{
		Name:             "default",
		Description:      "Default consent policy",
		RequiredPurposes: []Purpose{PurposeAgentExecution},
		OptionalPurposes: []Purpose{PurposeAnalytics},
		DataCategories:   []DataCategory{DataSourceCode},
		RetentionDays:    90,
	})
	if err != nil {
		t.Fatalf("CreatePolicy: %v", err)
	}
	if p.ID == "" {
		t.Fatal("expected policy ID")
	}
	if !p.Active {
		t.Fatal("expected active")
	}

	policies, _ := s.ListPolicies()
	if len(policies) != 1 {
		t.Fatalf("expected 1 policy, got %d", len(policies))
	}
}

func TestExportJSON(t *testing.T) {
	s := tempConsentStore(t)

	s.Grant("user-1", "", []Purpose{PurposeAgentExecution}, []DataCategory{DataSourceCode})

	data, err := s.ExportJSON()
	if err != nil {
		t.Fatalf("ExportJSON: %v", err)
	}

	var export struct {
		Records  []*Record `json:"records"`
		Exported time.Time `json:"exported"`
	}
	if err := json.Unmarshal(data, &export); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(export.Records) != 1 {
		t.Fatalf("expected 1 record in export, got %d", len(export.Records))
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	s1, _ := NewStore(dir)

	r1, _ := s1.Grant("user-1", "tenant-1",
		[]Purpose{PurposeAgentExecution},
		[]DataCategory{DataSourceCode},
		WithDescription("test"),
	)

	// Reload.
	s2, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore reload: %v", err)
	}

	r2, err := s2.Get(r1.ID)
	if err != nil {
		t.Fatalf("Get after reload: %v", err)
	}
	if r2.UserID != "user-1" {
		t.Fatalf("expected user-1, got %s", r2.UserID)
	}
	if r2.Description != "test" {
		t.Fatalf("expected description 'test', got %s", r2.Description)
	}
	if r2.Checksum != r1.Checksum {
		t.Fatal("checksum mismatch after reload")
	}

	// Verify persistence files exist.
	if _, err := os.Stat(filepath.Join(dir, "records.json")); err != nil {
		t.Fatalf("records.json missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "policies.json")); err != nil {
		t.Fatalf("policies.json missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "audit.json")); err != nil {
		t.Fatalf("audit.json missing: %v", err)
	}
}

func TestGetNonexistent(t *testing.T) {
	s := tempConsentStore(t)
	_, err := s.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent record")
	}
}

func TestRevokeNonexistent(t *testing.T) {
	s := tempConsentStore(t)
	_, err := s.Revoke("nonexistent", "test")
	if err == nil {
		t.Fatal("expected error for nonexistent record")
	}
}

func TestRevokeAlreadyRevoked(t *testing.T) {
	s := tempConsentStore(t)
	r, _ := s.Grant("user-1", "", []Purpose{PurposeAgentExecution}, []DataCategory{DataSourceCode})
	s.Revoke(r.ID, "first")
	_, err := s.Revoke(r.ID, "second")
	if err == nil {
		t.Fatal("expected error for double revoke")
	}
}
