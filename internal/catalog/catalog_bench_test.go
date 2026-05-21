package catalog

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func newBenchStore(b *testing.B) *Store {
	b.Helper()
	dir, err := os.MkdirTemp("", "catalog-bench-*")
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { os.RemoveAll(dir) })
	s, err := NewStore(dir)
	if err != nil {
		b.Fatal(err)
	}
	return s
}

func sampleEntry(name, ns string, t EntryType) Entry {
	schema, _ := json.Marshal(map[string]string{
		"input":  "string",
		"output": "string",
	})
	return Entry{
		Name:           name,
		Namespace:      ns,
		Type:           t,
		Description:    "Benchmark entry: " + name,
		Status:         StatusActive,
		Owner:          "bench-owner",
		Classification: ClassInternal,
		Tags:           []string{"bench", "test", string(t)},
		Schema:         schema,
	}
}

// BenchmarkRegister measures the cost of registering a new catalog entry.
func BenchmarkRegister(b *testing.B) {
	s := newBenchStore(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e := sampleEntry(fmt.Sprintf("agent-%d", i), "default", TypeAgent)
		s.Register(e) //nolint:errcheck
	}
}

// BenchmarkGet measures catalog entry retrieval by ID.
func BenchmarkGet(b *testing.B) {
	s := newBenchStore(b)
	// Pre-populate 500 entries
	ids := make([]string, 500)
	for i := 0; i < 500; i++ {
		e, err := s.Register(sampleEntry(fmt.Sprintf("tool-%d", i), "bench", TypeTool))
		if err != nil {
			b.Fatal(err)
		}
		ids[i] = e.ID
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Get(ids[i%500]) //nolint:errcheck
	}
}

// BenchmarkList_AllEntries measures full catalog listing with 500 entries.
func BenchmarkList_AllEntries(b *testing.B) {
	s := newBenchStore(b)
	for i := 0; i < 500; i++ {
		s.Register(sampleEntry(fmt.Sprintf("entry-%d", i), "ns", TypeAgent)) //nolint:errcheck
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.List(nil) //nolint:errcheck
	}
}

// BenchmarkList_Filtered measures listing with type filter.
func BenchmarkList_Filtered(b *testing.B) {
	s := newBenchStore(b)
	types := []EntryType{TypeAgent, TypeTool, TypeModel, TypeDataSource, TypePipeline}
	for i := 0; i < 500; i++ {
		s.Register(sampleEntry(fmt.Sprintf("e-%d", i), "ns", types[i%len(types)])) //nolint:errcheck
	}
	filter := map[string]string{"type": "agent"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.List(filter) //nolint:errcheck
	}
}

// BenchmarkSearch measures full-text search across name, description, and tags.
func BenchmarkSearch(b *testing.B) {
	s := newBenchStore(b)
	for i := 0; i < 500; i++ {
		s.Register(sampleEntry(fmt.Sprintf("forge-agent-%d", i), "default", TypeAgent)) //nolint:errcheck
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Search("forge") //nolint:errcheck
	}
}

// BenchmarkSearch_NoMatch measures search performance when nothing matches.
func BenchmarkSearch_NoMatch(b *testing.B) {
	s := newBenchStore(b)
	for i := 0; i < 500; i++ {
		s.Register(sampleEntry(fmt.Sprintf("agent-%d", i), "default", TypeAgent)) //nolint:errcheck
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Search("xyzzy-nonexistent-query-99999") //nolint:errcheck
	}
}

// BenchmarkUpdate measures catalog entry update performance.
func BenchmarkUpdate(b *testing.B) {
	s := newBenchStore(b)
	ids := make([]string, 100)
	for i := 0; i < 100; i++ {
		e, err := s.Register(sampleEntry(fmt.Sprintf("upd-agent-%d", i), "bench", TypeAgent))
		if err != nil {
			b.Fatal(err)
		}
		ids[i] = e.ID
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := ids[i%100]
		s.Update(id, func(e *Entry) { //nolint:errcheck
			e.Description = fmt.Sprintf("Updated in iteration %d", i)
			e.UpdatedBy = "bench"
		})
	}
}

// BenchmarkGetStats measures stats computation over a populated catalog.
func BenchmarkGetStats(b *testing.B) {
	s := newBenchStore(b)
	types := []EntryType{TypeAgent, TypeTool, TypeModel, TypeDataSource}
	ns := []string{"default", "ml", "ops", "finance"}
	for i := 0; i < 500; i++ {
		e := sampleEntry(fmt.Sprintf("e-%d", i), ns[i%len(ns)], types[i%len(types)])
		s.Register(e) //nolint:errcheck
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.GetStats()
	}
}

// BenchmarkMakeEntryID measures ID generation.
func BenchmarkMakeEntryID(b *testing.B) {
	combos := [][3]string{
		{"default", "my-agent", "v1"},
		{"", "tool-x", ""},
		{"ml", "embedding-model", "2.0"},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c := combos[i%len(combos)]
		makeEntryID(c[0], c[1], c[2])
	}
}

// BenchmarkComputeChecksum measures entry checksum computation.
func BenchmarkComputeChecksum(b *testing.B) {
	schema, _ := json.Marshal(map[string]interface{}{
		"input":  map[string]string{"type": "string"},
		"output": map[string]string{"type": "string"},
		"extra":  []string{"a", "b", "c"},
	})
	e := &Entry{
		ID:      "default/my-agent@v1",
		Name:    "my-agent",
		Type:    TypeAgent,
		Version: "v1",
		Schema:  schema,
		Metadata: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		computeEntryChecksum(e)
	}
}

// BenchmarkGetDependents measures reverse-dependency lookup over a large catalog.
func BenchmarkGetDependents(b *testing.B) {
	s := newBenchStore(b)
	// Register a shared "base" tool that many agents depend on
	base, err := s.Register(sampleEntry("base-tool", "shared", TypeTool))
	if err != nil {
		b.Fatal(err)
	}
	for i := 0; i < 200; i++ {
		e := sampleEntry(fmt.Sprintf("dep-agent-%d", i), "default", TypeAgent)
		if i%2 == 0 {
			e.Dependencies = []string{base.ID}
		}
		s.Register(e) //nolint:errcheck
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.GetDependents(base.ID) //nolint:errcheck
	}
}

// BenchmarkExportJSON measures full catalog JSON export.
func BenchmarkExportJSON(b *testing.B) {
	s := newBenchStore(b)
	for i := 0; i < 200; i++ {
		s.Register(sampleEntry(fmt.Sprintf("export-e-%d", i), "ns", TypeAgent)) //nolint:errcheck
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.ExportJSON() //nolint:errcheck
	}
}
