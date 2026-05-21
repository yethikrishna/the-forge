package catalog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func tempCatalogStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return s
}

func a(name string, overrides ...func(*Entry)) Entry {
	e := Entry{
		Type:      TypeAgent,
		Name:      name,
		Namespace: "default",
		Version:   "1.0.0",
		Owner:     "admin",
	}
	for _, o := range overrides {
		o(&e)
	}
	return e
}

func TestRegister(t *testing.T) {
	s := tempCatalogStore(t)

	e, err := s.Register(a("coder-agent"))
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if e.ID != "default/coder-agent@1.0.0" {
		t.Fatalf("unexpected ID: %s", e.ID)
	}
	if e.Status != StatusActive {
		t.Fatalf("expected active, got %s", e.Status)
	}
	if e.Checksum == "" {
		t.Fatal("expected checksum")
	}
	if e.CreatedAt.IsZero() {
		t.Fatal("expected created_at")
	}
}

func TestRegisterDuplicate(t *testing.T) {
	s := tempCatalogStore(t)
	s.Register(a("dup"))
	_, err := s.Register(a("dup"))
	if err == nil {
		t.Fatal("expected error for duplicate")
	}
}

func TestRegisterValidation(t *testing.T) {
	s := tempCatalogStore(t)
	_, err := s.Register(Entry{Type: TypeAgent})
	if err == nil {
		t.Fatal("expected error for missing name")
	}
	_, err = s.Register(Entry{Name: "test"})
	if err == nil {
		t.Fatal("expected error for missing type")
	}
}

func TestRegisterWithAllFields(t *testing.T) {
	s := tempCatalogStore(t)

	e, err := s.Register(Entry{
		Type:           TypeTool,
		Name:           "file-reader",
		Namespace:      "team-ml",
		Version:        "2.1.0",
		Description:    "Reads files from disk",
		Owner:          "alice",
		Classification: ClassConfidential,
		Tags:           []string{"file", "io", "reader"},
		Labels:         map[string]string{"env": "prod"},
		URI:            "internal/tools/file-reader",
		Schema:         json.RawMessage(`{"input": "path", "output": "bytes"}`),
		Metadata:       map[string]string{"runtime": "wasm"},
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if e.Classification != ClassConfidential {
		t.Fatal("wrong classification")
	}
	if len(e.Tags) != 3 {
		t.Fatalf("expected 3 tags, got %d", len(e.Tags))
	}
}

func TestGet(t *testing.T) {
	s := tempCatalogStore(t)
	reg, _ := s.Register(a("fetcher"))

	got, err := s.Get(reg.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "fetcher" {
		t.Fatalf("expected fetcher, got %s", got.Name)
	}
}

func TestGetByName(t *testing.T) {
	s := tempCatalogStore(t)
	s.Register(a("searcher"))

	got, err := s.GetByName("default", "searcher", "1.0.0")
	if err != nil {
		t.Fatalf("GetByName: %v", err)
	}
	if got.Name != "searcher" {
		t.Fatalf("expected searcher, got %s", got.Name)
	}
}

func TestGetNonexistent(t *testing.T) {
	s := tempCatalogStore(t)
	_, err := s.Get("nope")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUpdate(t *testing.T) {
	s := tempCatalogStore(t)
	reg, _ := s.Register(a("updater"))

	updated, err := s.Update(reg.ID, func(e *Entry) {
		e.Description = "updated description"
		e.Tags = []string{"new-tag"}
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Description != "updated description" {
		t.Fatal("description not updated")
	}
	if len(updated.Tags) != 1 || updated.Tags[0] != "new-tag" {
		t.Fatal("tags not updated")
	}
	if updated.UpdatedAt.Before(reg.CreatedAt) {
		t.Fatal("updated_at should be after created_at")
	}
}

func TestDelete(t *testing.T) {
	s := tempCatalogStore(t)
	reg, _ := s.Register(a("deleter"))

	if err := s.Delete(reg.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err := s.Get(reg.ID)
	if err == nil {
		t.Fatal("should be deleted")
	}
}

func TestList(t *testing.T) {
	s := tempCatalogStore(t)

	s.Register(Entry{Type: TypeAgent, Name: "agent-1", Namespace: "default", Owner: "alice"})
	s.Register(Entry{Type: TypeTool, Name: "tool-1", Namespace: "default", Owner: "bob"})
	s.Register(Entry{Type: TypeAgent, Name: "agent-2", Namespace: "team-ml", Owner: "alice"})

	// All.
	all, _ := s.List(map[string]string{})
	if len(all) != 3 {
		t.Fatalf("expected 3, got %d", len(all))
	}

	// By type.
	agents, _ := s.List(map[string]string{"type": "agent"})
	if len(agents) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(agents))
	}

	// By namespace.
	ml, _ := s.List(map[string]string{"namespace": "team-ml"})
	if len(ml) != 1 {
		t.Fatalf("expected 1 ml entry, got %d", len(ml))
	}

	// By owner.
	alice, _ := s.List(map[string]string{"owner": "alice"})
	if len(alice) != 2 {
		t.Fatalf("expected 2 alice entries, got %d", len(alice))
	}

	// By tag.
	s.Register(Entry{Type: TypeAgent, Name: "tagged", Namespace: "ns", Tags: []string{"prod"}})
	tagged, _ := s.List(map[string]string{"tag": "prod"})
	if len(tagged) != 1 {
		t.Fatalf("expected 1 tagged, got %d", len(tagged))
	}
}

func TestSearch(t *testing.T) {
	s := tempCatalogStore(t)

	s.Register(Entry{Type: TypeAgent, Name: "code-reviewer", Namespace: "default", Description: "Reviews code for quality"})
	s.Register(Entry{Type: TypeAgent, Name: "doc-writer", Namespace: "default", Description: "Writes documentation"})
	s.Register(Entry{Type: TypeTool, Name: "search", Namespace: "default", Tags: []string{"code-search"}})

	// Search by name.
	results, _ := s.Search("reviewer")
	if len(results) != 1 {
		t.Fatalf("expected 1, got %d", len(results))
	}

	// Search by description.
	results, _ = s.Search("documentation")
	if len(results) != 1 {
		t.Fatalf("expected 1 doc result, got %d", len(results))
	}

	// Search by tag.
	results, _ = s.Search("code-search")
	if len(results) != 1 {
		t.Fatalf("expected 1 tag result, got %d", len(results))
	}

	// Case insensitive.
	results, _ = s.Search("REVIEWER")
	if len(results) != 1 {
		t.Fatalf("expected case-insensitive match, got %d", len(results))
	}
}

func TestDependencies(t *testing.T) {
	s := tempCatalogStore(t)

	tool, _ := s.Register(Entry{Type: TypeTool, Name: "git-tool", Namespace: "default", Version: "1.0"})
	agent, _ := s.Register(Entry{
		Type:         TypeAgent,
		Name:         "code-agent",
		Namespace:    "default",
		Version:      "1.0",
		Dependencies: []string{tool.ID},
	})

	deps, err := s.GetDependencies(agent.ID)
	if err != nil {
		t.Fatalf("GetDependencies: %v", err)
	}
	if len(deps) != 1 || deps[0].Name != "git-tool" {
		t.Fatal("wrong dependencies")
	}

	dependents, err := s.GetDependents(tool.ID)
	if err != nil {
		t.Fatalf("GetDependents: %v", err)
	}
	if len(dependents) != 1 || dependents[0].Name != "code-agent" {
		t.Fatal("wrong dependents")
	}
}

func TestTransfer(t *testing.T) {
	s := tempCatalogStore(t)
	reg, _ := s.Register(a("transfer-test"))

	transferred, err := s.Transfer(reg.ID, "new-owner", "admin")
	if err != nil {
		t.Fatalf("Transfer: %v", err)
	}
	if transferred.Owner != "new-owner" {
		t.Fatalf("expected new-owner, got %s", transferred.Owner)
	}
}

func TestDeprecate(t *testing.T) {
	s := tempCatalogStore(t)
	reg, _ := s.Register(a("deprecate-test"))

	deprecated, err := s.Deprecate(reg.ID, "default/new-agent@2.0", "admin")
	if err != nil {
		t.Fatalf("Deprecate: %v", err)
	}
	if deprecated.Status != StatusArchived {
		t.Fatalf("expected archived, got %s", deprecated.Status)
	}
	if deprecated.Metadata["deprecated_by"] != "default/new-agent@2.0" {
		t.Fatal("replacement not recorded")
	}
}

func TestGetStats(t *testing.T) {
	s := tempCatalogStore(t)

	s.Register(Entry{Type: TypeAgent, Name: "a1", Namespace: "ns1", Owner: "alice", Tags: []string{"prod"}})
	s.Register(Entry{Type: TypeTool, Name: "t1", Namespace: "ns1", Classification: ClassConfidential, Tags: []string{"prod", "internal"}})
	s.Register(Entry{Type: TypeModel, Name: "m1", Namespace: "ns2"})

	stats := s.GetStats()
	if stats.TotalEntries != 3 {
		t.Fatalf("expected 3, got %d", stats.TotalEntries)
	}
	if stats.EntriesByType[TypeAgent] != 1 {
		t.Error("wrong agent count")
	}
	if stats.EntriesByType[TypeTool] != 1 {
		t.Error("wrong tool count")
	}
	if stats.EntriesByType[TypeModel] != 1 {
		t.Error("wrong model count")
	}
	if len(stats.Namespaces) != 2 {
		t.Fatalf("expected 2 namespaces, got %v", stats.Namespaces)
	}
	if len(stats.Tags) != 2 {
		t.Fatalf("expected 2 unique tags, got %d", len(stats.Tags))
	}
	if stats.AuditLogEntries < 3 {
		t.Fatalf("expected >= 3 audit entries, got %d", stats.AuditLogEntries)
	}
}

func TestAuditLog(t *testing.T) {
	s := tempCatalogStore(t)

	reg, _ := s.Register(Entry{Type: TypeAgent, Name: "audit-test", Namespace: "default", CreatedBy: "alice"})
	s.Transfer(reg.ID, "bob", "admin")

	logs, err := s.GetAuditLog(reg.ID)
	if err != nil {
		t.Fatalf("GetAuditLog: %v", err)
	}
	if len(logs) < 2 {
		t.Fatalf("expected >= 2 log entries, got %d", len(logs))
	}

	// Check first entry is register.
	if logs[0].Action != "register" {
		t.Fatalf("expected register first, got %s", logs[0].Action)
	}

	// All logs.
	allLogs, _ := s.GetAuditLog("")
	if len(allLogs) < 2 {
		t.Fatalf("expected >= 2 total logs, got %d", len(allLogs))
	}
}

func TestExportJSON(t *testing.T) {
	s := tempCatalogStore(t)
	s.Register(Entry{Type: TypeAgent, Name: "export-test", Namespace: "default"})

	data, err := s.ExportJSON()
	if err != nil {
		t.Fatalf("ExportJSON: %v", err)
	}

	var export struct {
		Entries  []*Entry    `json:"entries"`
		Audit    []*AuditLog `json:"audit_log"`
		Exported time.Time   `json:"exported"`
	}
	if err := json.Unmarshal(data, &export); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(export.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(export.Entries))
	}
	if export.Exported.IsZero() {
		t.Fatal("expected exported timestamp")
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	s1, _ := NewStore(dir)

	e1, _ := s1.Register(Entry{
		Type:        TypeAgent,
		Name:        "persist-test",
		Namespace:   "default",
		Version:     "1.0",
		Description: "tests persistence",
		Owner:       "alice",
		Tags:        []string{"test"},
	})

	// Flush before reload so write-behind cache has written to disk.
	if err := s1.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	s1.Close()

	s2, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore reload: %v", err)
	}

	e2, err := s2.Get(e1.ID)
	if err != nil {
		t.Fatalf("Get after reload: %v", err)
	}
	if e2.Name != "persist-test" {
		t.Fatalf("expected persist-test, got %s", e2.Name)
	}
	if e2.Description != "tests persistence" {
		t.Fatal("description not persisted")
	}
	if e2.Checksum != e1.Checksum {
		t.Fatal("checksum mismatch after reload")
	}

	if _, err := os.Stat(filepath.Join(dir, "entries.json")); err != nil {
		t.Fatalf("entries.json missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "audit.json")); err != nil {
		t.Fatalf("audit.json missing: %v", err)
	}
}

func TestEntryIDFormats(t *testing.T) {
	tests := []struct {
		ns, name, version, expected string
	}{
		{"default", "agent", "1.0", "default/agent@1.0"},
		{"", "agent", "1.0", "agent@1.0"},
		{"ns", "agent", "", "ns/agent"},
		{"", "agent", "", "agent"},
	}
	for _, tt := range tests {
		got := makeEntryID(tt.ns, tt.name, tt.version)
		if got != tt.expected {
			t.Errorf("makeEntryID(%q,%q,%q) = %q, want %q", tt.ns, tt.name, tt.version, got, tt.expected)
		}
	}
}

func TestSearchEmpty(t *testing.T) {
	s := tempCatalogStore(t)
	s.Register(Entry{Type: TypeAgent, Name: "test", Namespace: "default"})

	results, _ := s.Search("nonexistent")
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

func TestUpdateNonexistent(t *testing.T) {
	s := tempCatalogStore(t)
	_, err := s.Update("nonexistent", func(e *Entry) {})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestTransferNonexistent(t *testing.T) {
	s := tempCatalogStore(t)
	_, err := s.Transfer("nonexistent", "bob", "admin")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLargeCatalog(t *testing.T) {
	s := tempCatalogStore(t)

	for i := 0; i < 100; i++ {
		typ := TypeAgent
		if i%3 == 0 {
			typ = TypeTool
		} else if i%3 == 1 {
			typ = TypeModel
		}
		s.Register(Entry{
			Type:      typ,
			Name:      fmt.Sprintf("entry-%d", i),
			Namespace: fmt.Sprintf("ns-%d", i%5),
		})
	}

	stats := s.GetStats()
	if stats.TotalEntries != 100 {
		t.Fatalf("expected 100, got %d", stats.TotalEntries)
	}
	if len(stats.Namespaces) != 5 {
		t.Fatalf("expected 5 namespaces, got %d", len(stats.Namespaces))
	}

	agents, _ := s.List(map[string]string{"type": "agent"})
	if len(agents) < 30 {
		t.Fatalf("expected ~33 agents, got %d", len(agents))
	}
}

func TestClassificationFilter(t *testing.T) {
	s := tempCatalogStore(t)
	s.Register(Entry{Type: TypeAgent, Name: "public-agent", Namespace: "default", Classification: ClassPublic})
	s.Register(Entry{Type: TypeAgent, Name: "secret-agent", Namespace: "default", Classification: ClassRestricted})

	restricted, _ := s.List(map[string]string{"classification": "restricted"})
	if len(restricted) != 1 || restricted[0].Name != "secret-agent" {
		t.Fatal("classification filter failed")
	}
}

func TestStatusFilter(t *testing.T) {
	s := tempCatalogStore(t)
	reg, _ := s.Register(Entry{Type: TypeAgent, Name: "draft", Namespace: "default", Status: StatusDraft})
	s.Register(Entry{Type: TypeAgent, Name: "active", Namespace: "ns", Status: StatusActive})

	s.Deprecate(reg.ID, "", "admin")

	archived, _ := s.List(map[string]string{"status": "archived"})
	if len(archived) != 1 {
		t.Fatalf("expected 1 archived, got %d", len(archived))
	}
}
