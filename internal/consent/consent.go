// Package consent provides data usage consent management with consent receipts.
// It tracks what data is collected, why, who consented, and when — enabling
// GDPR compliance and transparent data practices.
//
// Consent is not a checkbox. It's a record.
package consent

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Purpose describes why data is being processed.
type Purpose string

const (
	PurposeAgentExecution Purpose = "agent_execution" // Running agents on user data
	PurposeMemory         Purpose = "memory"          // Storing agent memory
	PurposeAnalytics      Purpose = "analytics"       // Usage analytics
	PurposeTelemetry      Purpose = "telemetry"       // Telemetry data
	PurposeCostTracking   Purpose = "cost_tracking"   // Cost tracking
	PurposeAudit          Purpose = "audit"           // Audit logging
	PurposeTraining       Purpose = "training"        // Model training data
	PurposeSharing        Purpose = "sharing"         // Sharing with third parties
	PurposeIndexing       Purpose = "indexing"        // Codebase indexing
	PurposeCompliance     Purpose = "compliance"      // Compliance reporting
	PurposeCustom         Purpose = "custom"          // Custom purpose
)

// DataCategory describes the type of data being processed.
type DataCategory string

const (
	DataSourceCode    DataCategory = "source_code"
	DataConversations DataCategory = "conversations"
	DataAgentOutput   DataCategory = "agent_output"
	DataFileContent   DataCategory = "file_content"
	DataMetrics       DataCategory = "metrics"
	DataPersonal      DataCategory = "personal"
	DataCredentials   DataCategory = "credentials"
	DataIndex         DataCategory = "index"
	DataCustom        DataCategory = "custom"
)

// Status represents the state of a consent record.
type Status string

const (
	StatusGranted   Status = "granted"
	StatusRevoked   Status = "revoked"
	StatusExpired   Status = "expired"
	StatusPending   Status = "pending"
	StatusWithdrawn Status = "withdrawn"
)

// Record represents a single consent receipt.
type Record struct {
	ID               string            `json:"id"`
	UserID           string            `json:"user_id"`
	TenantID         string            `json:"tenant_id,omitempty"`
	Purposes         []Purpose         `json:"purposes"`
	DataCategories   []DataCategory    `json:"data_categories"`
	Status           Status            `json:"status"`
	GrantedAt        time.Time         `json:"granted_at"`
	RevokedAt        *time.Time        `json:"revoked_at,omitempty"`
	ExpiresAt        *time.Time        `json:"expires_at,omitempty"`
	Source           string            `json:"source"` // cli, api, ui, imported
	Description      string            `json:"description,omitempty"`
	LegalBasis       string            `json:"legal_basis,omitempty"` // consent, legitimate_interest, contract, etc.
	WithdrawalReason string            `json:"withdrawal_reason,omitempty"`
	Checksum         string            `json:"checksum"`      // Tamper-evidence
	PrevChecksum     string            `json:"prev_checksum"` // Hash chain
	Labels           map[string]string `json:"labels,omitempty"`
}

// Policy defines a consent policy that applies to a tenant or user.
type Policy struct {
	ID                string         `json:"id"`
	Name              string         `json:"name"`
	Description       string         `json:"description"`
	RequiredPurposes  []Purpose      `json:"required_purposes"`
	OptionalPurposes  []Purpose      `json:"optional_purposes"`
	DataCategories    []DataCategory `json:"data_categories"`
	RetentionDays     int            `json:"retention_days,omitempty"`
	DefaultExpiryDays int            `json:"default_expiry_days,omitempty"`
	AutoRevoke        bool           `json:"auto_revoke"` // Revoke on policy change
	Active            bool           `json:"active"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
}

// AuditEntry tracks consent changes for the audit log.
type AuditEntry struct {
	ID         string    `json:"id"`
	RecordID   string    `json:"record_id"`
	Action     string    `json:"action"` // grant, revoke, withdraw, expire, update
	UserID     string    `json:"user_id"`
	Timestamp  time.Time `json:"timestamp"`
	Details    string    `json:"details,omitempty"`
	PrevStatus Status    `json:"prev_status"`
	NewStatus  Status    `json:"new_status"`
}

// Stats holds aggregate consent statistics.
type Stats struct {
	TotalRecords     int                  `json:"total_records"`
	GrantedCount     int                  `json:"granted_count"`
	RevokedCount     int                  `json:"revoked_count"`
	PendingCount     int                  `json:"pending_count"`
	ExpiredCount     int                  `json:"expired_count"`
	WithdrawnCount   int                  `json:"withdrawn_count"`
	PurposeBreakdown map[Purpose]int      `json:"purpose_breakdown"`
	DataBreakdown    map[DataCategory]int `json:"data_breakdown"`
	AuditTrailCount  int                  `json:"audit_trail_count"`
}

// Store manages consent records and policies.
type Store struct {
	Dir        string
	mu         sync.RWMutex
	records    map[string]*Record
	policies   map[string]*Policy
	auditTrail []*AuditEntry
	lastHash   string
}

// NewStore creates or loads a consent store.
func NewStore(dir string) (*Store, error) {
	s := &Store{
		Dir:      dir,
		records:  make(map[string]*Record),
		policies: make(map[string]*Policy),
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create consent dir: %w", err)
	}
	if err := s.load(); err != nil {
		return s, nil // Fresh store.
	}
	return s, nil
}

// generateID creates a unique ID.
func generateConsentID(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
	}
	h.Write([]byte(time.Now().Format(time.RFC3339Nano)))
	return fmt.Sprintf("csr-%x", h.Sum(nil))[:20]
}

// checksum computes a tamper-evident hash for a record.
func checksumRecord(r *Record, prevHash string) string {
	h := sha256.New()
	h.Write([]byte(r.ID))
	h.Write([]byte(r.UserID))
	h.Write([]byte(string(r.Status)))
	h.Write([]byte(r.GrantedAt.Format(time.RFC3339Nano)))
	for _, p := range r.Purposes {
		h.Write([]byte(p))
	}
	h.Write([]byte(prevHash))
	return fmt.Sprintf("%x", h.Sum(nil))[:32]
}

// Grant creates a new consent record.
func (s *Store) Grant(userID, tenantID string, purposes []Purpose, categories []DataCategory, opts ...GrantOption) (*Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	r := &Record{
		ID:             generateConsentID(userID, string(purposes[0])),
		UserID:         userID,
		TenantID:       tenantID,
		Purposes:       purposes,
		DataCategories: categories,
		Status:         StatusGranted,
		GrantedAt:      now,
		Source:         "api",
		LegalBasis:     "consent",
		Labels:         make(map[string]string),
	}

	for _, opt := range opts {
		opt(r)
	}

	if r.Source == "" {
		r.Source = "api"
	}

	r.Checksum = checksumRecord(r, s.lastHash)
	r.PrevChecksum = s.lastHash
	s.lastHash = r.Checksum

	s.records[r.ID] = r
	s.addAudit(r.ID, "grant", userID, "", StatusGranted)

	if err := s.save(); err != nil {
		return nil, err
	}
	return r, nil
}

// GrantOption configures optional fields on a consent record.
type GrantOption func(*Record)

// WithExpiry sets an expiry time.
func WithExpiry(d time.Duration) GrantOption {
	return func(r *Record) {
		t := r.GrantedAt.Add(d)
		r.ExpiresAt = &t
	}
}

// WithDescription sets a description.
func WithDescription(desc string) GrantOption {
	return func(r *Record) { r.Description = desc }
}

// WithSource sets the consent source.
func WithSource(source string) GrantOption {
	return func(r *Record) { r.Source = source }
}

// WithLegalBasis sets the legal basis.
func WithLegalBasis(basis string) GrantOption {
	return func(r *Record) { r.LegalBasis = basis }
}

// WithLabels sets labels.
func WithLabels(labels map[string]string) GrantOption {
	return func(r *Record) {
		if r.Labels == nil {
			r.Labels = make(map[string]string)
		}
		for k, v := range labels {
			r.Labels[k] = v
		}
	}
}

// Revoke revokes a consent record.
func (s *Store) Revoke(recordID, reason string) (*Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	r, ok := s.records[recordID]
	if !ok {
		return nil, fmt.Errorf("consent record %s not found", recordID)
	}
	if r.Status != StatusGranted && r.Status != StatusPending {
		return nil, fmt.Errorf("cannot revoke record in status %s", r.Status)
	}

	prevStatus := r.Status
	now := time.Now().UTC()
	r.Status = StatusRevoked
	r.RevokedAt = &now
	r.WithdrawalReason = reason
	r.Checksum = checksumRecord(r, s.lastHash)
	r.PrevChecksum = s.lastHash
	s.lastHash = r.Checksum

	s.addAudit(r.ID, "revoke", r.UserID, reason, prevStatus)

	if err := s.save(); err != nil {
		return nil, err
	}
	return r, nil
}

// Withdraw allows a user to withdraw consent.
func (s *Store) Withdraw(recordID, userID, reason string) (*Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	r, ok := s.records[recordID]
	if !ok {
		return nil, fmt.Errorf("consent record %s not found", recordID)
	}
	if r.UserID != userID {
		return nil, fmt.Errorf("only the consenting user can withdraw")
	}
	if r.Status != StatusGranted {
		return nil, fmt.Errorf("can only withdraw granted consent")
	}

	prevStatus := r.Status
	now := time.Now().UTC()
	r.Status = StatusWithdrawn
	r.RevokedAt = &now
	r.WithdrawalReason = reason
	r.Checksum = checksumRecord(r, s.lastHash)
	r.PrevChecksum = s.lastHash
	s.lastHash = r.Checksum

	s.addAudit(r.ID, "withdraw", userID, reason, prevStatus)

	if err := s.save(); err != nil {
		return nil, err
	}
	return r, nil
}

// Get retrieves a consent record by ID.
func (s *Store) Get(recordID string) (*Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	r, ok := s.records[recordID]
	if !ok {
		return nil, fmt.Errorf("consent record %s not found", recordID)
	}
	return r, nil
}

// Check verifies if consent is currently granted for a user/purpose combination.
func (s *Store) Check(userID string, purpose Purpose) (bool, *Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now().UTC()

	// Find the most recent granted record matching the criteria.
	var best *Record
	for _, r := range s.records {
		if r.UserID != userID {
			continue
		}
		if r.Status != StatusGranted {
			continue
		}
		if r.ExpiresAt != nil && r.ExpiresAt.Before(now) {
			continue
		}
		for _, p := range r.Purposes {
			if p == purpose {
				if best == nil || r.GrantedAt.After(best.GrantedAt) {
					best = r
				}
			}
		}
	}

	return best != nil, best, nil
}

// List returns consent records matching filters.
func (s *Store) List(filters map[string]string) ([]*Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*Record
	for _, r := range s.records {
		if matchesConsentFilters(r, filters) {
			results = append(results, r)
		}
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].GrantedAt.After(results[j].GrantedAt)
	})
	return results, nil
}

// ListByUser returns all consent records for a user.
func (s *Store) ListByUser(userID string) ([]*Record, error) {
	return s.List(map[string]string{"user_id": userID})
}

// Expire marks expired records.
func (s *Store) Expire() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	var count int

	for _, r := range s.records {
		if r.Status != StatusGranted {
			continue
		}
		if r.ExpiresAt != nil && r.ExpiresAt.Before(now) {
			prevStatus := r.Status
			r.Status = StatusExpired
			r.RevokedAt = &now
			r.Checksum = checksumRecord(r, s.lastHash)
			r.PrevChecksum = s.lastHash
			s.lastHash = r.Checksum
			s.addAudit(r.ID, "expire", r.UserID, "automatic expiry", prevStatus)
			count++
		}
	}

	if count > 0 {
		if err := s.save(); err != nil {
			return 0, err
		}
	}
	return count, nil
}

// Verify checks the integrity of the consent hash chain.
func (s *Store) Verify() (bool, []string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var issues []string
	prevHash := ""

	// Sort records by granted time for chain verification.
	var sorted []*Record
	for _, r := range s.records {
		sorted = append(sorted, r)
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].GrantedAt.Before(sorted[j].GrantedAt)
	})

	for _, r := range sorted {
		expected := checksumRecord(r, prevHash)
		if r.Checksum != expected {
			issues = append(issues, fmt.Sprintf("checksum mismatch for %s: expected %s, got %s", r.ID, expected, r.Checksum))
		}
		if r.PrevChecksum != prevHash {
			issues = append(issues, fmt.Sprintf("chain break at %s: prev_hash mismatch", r.ID))
		}
		prevHash = r.Checksum
	}

	return len(issues) == 0, issues, nil
}

// GetStats returns aggregate statistics.
func (s *Store) GetStats() *Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &Stats{
		PurposeBreakdown: make(map[Purpose]int),
		DataBreakdown:    make(map[DataCategory]int),
	}

	for _, r := range s.records {
		stats.TotalRecords++
		switch r.Status {
		case StatusGranted:
			stats.GrantedCount++
		case StatusRevoked:
			stats.RevokedCount++
		case StatusPending:
			stats.PendingCount++
		case StatusExpired:
			stats.ExpiredCount++
		case StatusWithdrawn:
			stats.WithdrawnCount++
		}
		for _, p := range r.Purposes {
			stats.PurposeBreakdown[p]++
		}
		for _, c := range r.DataCategories {
			stats.DataBreakdown[c]++
		}
	}
	stats.AuditTrailCount = len(s.auditTrail)

	return stats
}

// GetAuditTrail returns the consent audit trail.
func (s *Store) GetAuditTrail(recordID string) ([]*AuditEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*AuditEntry
	for _, e := range s.auditTrail {
		if recordID == "" || e.RecordID == recordID {
			results = append(results, e)
		}
	}
	return results, nil
}

// CreatePolicy creates a consent policy.
func (s *Store) CreatePolicy(policy Policy) (*Policy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if policy.ID == "" {
		policy.ID = generateConsentID("policy", policy.Name)
	}
	now := time.Now().UTC()
	policy.CreatedAt = now
	policy.UpdatedAt = now
	policy.Active = true

	s.policies[policy.ID] = &policy
	if err := s.save(); err != nil {
		return nil, err
	}
	return &policy, nil
}

// ListPolicies returns all policies.
func (s *Store) ListPolicies() ([]*Policy, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*Policy
	for _, p := range s.policies {
		results = append(results, p)
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].CreatedAt.After(results[j].CreatedAt)
	})
	return results, nil
}

// ExportJSON exports all consent data as JSON.
func (s *Store) ExportJSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	export := struct {
		Records  []*Record     `json:"records"`
		Policies []*Policy     `json:"policies"`
		Audit    []*AuditEntry `json:"audit_trail"`
		Exported time.Time     `json:"exported"`
	}{
		Exported: time.Now().UTC(),
	}
	for _, r := range s.records {
		export.Records = append(export.Records, r)
	}
	for _, p := range s.policies {
		export.Policies = append(export.Policies, p)
	}
	export.Audit = s.auditTrail

	return json.MarshalIndent(export, "", "  ")
}

// --- internal ---

func (s *Store) addAudit(recordID, action, userID, details string, prevStatus Status) {
	entry := &AuditEntry{
		ID:         generateConsentID("audit", recordID, action),
		RecordID:   recordID,
		Action:     action,
		UserID:     userID,
		Timestamp:  time.Now().UTC(),
		Details:    details,
		PrevStatus: prevStatus,
		NewStatus: func() Status {
			switch action {
			case "grant":
				return StatusGranted
			case "revoke":
				return StatusRevoked
			case "withdraw":
				return StatusWithdrawn
			case "expire":
				return StatusExpired
			default:
				return ""
			}
		}(),
	}
	s.auditTrail = append(s.auditTrail, entry)
}

func matchesConsentFilters(r *Record, filters map[string]string) bool {
	for k, v := range filters {
		switch k {
		case "user_id":
			if r.UserID != v {
				return false
			}
		case "tenant_id":
			if r.TenantID != v {
				return false
			}
		case "status":
			if string(r.Status) != v {
				return false
			}
		case "source":
			if r.Source != v {
				return false
			}
		case "purpose":
			found := false
			for _, p := range r.Purposes {
				if string(p) == v {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}
	return true
}

// --- persistence ---

func (s *Store) load() error {
	recordsPath := filepath.Join(s.Dir, "records.json")
	policiesPath := filepath.Join(s.Dir, "policies.json")
	auditPath := filepath.Join(s.Dir, "audit.json")

	if data, err := os.ReadFile(recordsPath); err == nil {
		if err := json.Unmarshal(data, &s.records); err != nil {
			return fmt.Errorf("unmarshal records: %w", err)
		}
	}

	if data, err := os.ReadFile(policiesPath); err == nil {
		if err := json.Unmarshal(data, &s.policies); err != nil {
			return fmt.Errorf("unmarshal policies: %w", err)
		}
	}

	if data, err := os.ReadFile(auditPath); err == nil {
		if err := json.Unmarshal(data, &s.auditTrail); err != nil {
			return fmt.Errorf("unmarshal audit: %w", err)
		}
	}

	// Rebuild last hash from records.
	var sorted []*Record
	for _, r := range s.records {
		sorted = append(sorted, r)
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].GrantedAt.Before(sorted[j].GrantedAt)
	})
	if len(sorted) > 0 {
		s.lastHash = sorted[len(sorted)-1].Checksum
	}

	return nil
}

func (s *Store) save() error {
	recordsData, err := json.MarshalIndent(s.records, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal records: %w", err)
	}
	policiesData, err := json.MarshalIndent(s.policies, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal policies: %w", err)
	}
	auditData, err := json.MarshalIndent(s.auditTrail, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal audit: %w", err)
	}

	if err := os.WriteFile(filepath.Join(s.Dir, "records.json"), recordsData, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(s.Dir, "policies.json"), policiesData, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(s.Dir, "audit.json"), auditData, 0o644); err != nil {
		return err
	}
	return nil
}
