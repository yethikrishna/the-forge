package traces

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTraceStore(t *testing.T) {
	dir := t.TempDir()
	store := NewTraceStore(dir)

	now := time.Now().UTC()
	span := StoredSpan{
		TraceID:  "trace001",
		SpanID:   "span001",
		Name:     "forge.run",
		Kind:     "client",
		Status:   "ok",
		Start:    now,
		End:      now.Add(100 * time.Millisecond),
		Duration: 100 * time.Millisecond,
		Service:  "forge",
		Attrs:    map[string]string{"model": "gpt-4.1"},
	}

	if err := store.Store(span); err != nil {
		t.Fatalf("Store: %v", err)
	}

	summaries, err := store.ListTraces(ListOpts{})
	if err != nil {
		t.Fatalf("ListTraces: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("expected 1 trace, got %d", len(summaries))
	}
	if summaries[0].TraceID != "trace001" {
		t.Fatalf("expected trace001, got %s", summaries[0].TraceID)
	}
	if summaries[0].SpanCount != 1 {
		t.Fatalf("expected 1 span, got %d", summaries[0].SpanCount)
	}
}

func TestTraceStoreGetDetail(t *testing.T) {
	dir := t.TempDir()
	store := NewTraceStore(dir)

	now := time.Now().UTC()
	parent := StoredSpan{
		TraceID:  "trace002",
		SpanID:   "span-parent",
		Name:     "forge.orchestrate",
		Kind:     "server",
		Status:   "ok",
		Start:    now,
		End:      now.Add(200 * time.Millisecond),
		Duration: 200 * time.Millisecond,
		Service:  "forge",
	}
	child := StoredSpan{
		TraceID:  "trace002",
		SpanID:   "span-child",
		ParentID: "span-parent",
		Name:     "forge.run",
		Kind:     "client",
		Status:   "ok",
		Start:    now.Add(10 * time.Millisecond),
		End:      now.Add(150 * time.Millisecond),
		Duration: 140 * time.Millisecond,
		Service:  "forge",
	}

	store.StoreBatch([]StoredSpan{parent, child})

	detail, err := store.GetTrace("trace002")
	if err != nil {
		t.Fatalf("GetTrace: %v", err)
	}
	if detail.Summary.SpanCount != 2 {
		t.Fatalf("expected 2 spans, got %d", detail.Summary.SpanCount)
	}
	if detail.Tree.Span.SpanID != "span-parent" {
		t.Fatalf("expected root span-parent, got %s", detail.Tree.Span.SpanID)
	}
	if len(detail.Tree.Children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(detail.Tree.Children))
	}
}

func TestTraceStoreFilters(t *testing.T) {
	dir := t.TempDir()
	store := NewTraceStore(dir)

	now := time.Now().UTC()
	spans := []StoredSpan{
		{TraceID: "t1", SpanID: "s1", Name: "forge.run", Status: "ok", Service: "forge", Start: now, Duration: 50 * time.Millisecond},
		{TraceID: "t2", SpanID: "s2", Name: "forge.review", Status: "error", Service: "forge-review", Start: now.Add(time.Hour), Duration: 200 * time.Millisecond},
	}
	store.StoreBatch(spans)

	// Filter by service
	results, _ := store.ListTraces(ListOpts{Service: "forge-review"})
	if len(results) != 1 || results[0].TraceID != "t2" {
		t.Fatalf("service filter: expected t2, got %v", results)
	}

	// Filter by status
	results, _ = store.ListTraces(ListOpts{Status: "error"})
	if len(results) != 1 || results[0].TraceID != "t2" {
		t.Fatalf("status filter: expected t2, got %v", results)
	}
}

func TestJaegerExport(t *testing.T) {
	dir := t.TempDir()
	store := NewTraceStore(dir)

	now := time.Now().UTC()
	store.Store(StoredSpan{
		TraceID: "t1", SpanID: "s1", Name: "forge.run", Status: "ok",
		Service: "forge", Start: now, End: now.Add(50 * time.Millisecond), Duration: 50 * time.Millisecond,
	})

	jtraces, err := store.ExportJaeger(ListOpts{})
	if err != nil {
		t.Fatalf("ExportJaeger: %v", err)
	}
	if len(jtraces) != 1 {
		t.Fatalf("expected 1 Jaeger trace, got %d", len(jtraces))
	}

	data, _ := json.Marshal(jtraces[0])
	if len(data) == 0 {
		t.Fatal("empty Jaeger export")
	}
}

func TestZipkinExport(t *testing.T) {
	dir := t.TempDir()
	store := NewTraceStore(dir)

	now := time.Now().UTC()
	store.Store(StoredSpan{
		TraceID: "t1", SpanID: "s1", Name: "forge.run", Status: "ok",
		Service: "forge", Start: now, End: now.Add(50 * time.Millisecond), Duration: 50 * time.Millisecond,
		Attrs: map[string]string{"model": "gpt-4.1"},
	})

	zspans, err := store.ExportZipkin(ListOpts{})
	if err != nil {
		t.Fatalf("ExportZipkin: %v", err)
	}
	if len(zspans) != 1 {
		t.Fatalf("expected 1 Zipkin span, got %d", len(zspans))
	}
	if zspans[0].LocalEndpoint.ServiceName != "forge" {
		t.Fatalf("expected forge service, got %s", zspans[0].LocalEndpoint.ServiceName)
	}
}

func TestOTLPExport(t *testing.T) {
	dir := t.TempDir()
	store := NewTraceStore(dir)

	now := time.Now().UTC()
	store.Store(StoredSpan{
		TraceID: "t1", SpanID: "s1", Name: "forge.run", Status: "ok",
		Service: "forge", Kind: "client", Start: now, End: now.Add(50 * time.Millisecond), Duration: 50 * time.Millisecond,
	})

	export, err := store.ExportOpenTelemetry(ListOpts{})
	if err != nil {
		t.Fatalf("ExportOpenTelemetry: %v", err)
	}
	if len(export.ResourceSpans) != 1 {
		t.Fatalf("expected 1 resource span, got %d", len(export.ResourceSpans))
	}
}

func TestTraceStoreStats(t *testing.T) {
	dir := t.TempDir()
	store := NewTraceStore(dir)

	now := time.Now().UTC()
	store.StoreBatch([]StoredSpan{
		{TraceID: "t1", SpanID: "s1", Name: "forge.run", Status: "ok", Service: "forge", Start: now, Duration: 50 * time.Millisecond},
		{TraceID: "t1", SpanID: "s2", Name: "forge.build", Status: "error", Service: "forge", Start: now, Duration: 30 * time.Millisecond},
		{TraceID: "t2", SpanID: "s3", Name: "forge.review", Status: "ok", Service: "review", Start: now, Duration: 100 * time.Millisecond},
	})

	stats := store.Stats()
	if stats.TotalTraces != 2 {
		t.Fatalf("expected 2 traces, got %d", stats.TotalTraces)
	}
	if stats.TotalSpans != 3 {
		t.Fatalf("expected 3 spans, got %d", stats.TotalSpans)
	}
	if stats.ErrorSpans != 1 {
		t.Fatalf("expected 1 error span, got %d", stats.ErrorSpans)
	}
}

func TestTraceStoreDelete(t *testing.T) {
	dir := t.TempDir()
	store := NewTraceStore(dir)

	now := time.Now().UTC()
	store.StoreBatch([]StoredSpan{
		{TraceID: "t1", SpanID: "s1", Name: "forge.run", Status: "ok", Service: "forge", Start: now, Duration: 50 * time.Millisecond},
		{TraceID: "t2", SpanID: "s2", Name: "forge.review", Status: "ok", Service: "forge", Start: now, Duration: 100 * time.Millisecond},
	})

	if err := store.DeleteTrace("t1"); err != nil {
		t.Fatalf("DeleteTrace: %v", err)
	}

	summaries, _ := store.ListTraces(ListOpts{})
	if len(summaries) != 1 || summaries[0].TraceID != "t2" {
		t.Fatalf("expected only t2 after delete, got %v", summaries)
	}
}

func TestTraceStorePersistence(t *testing.T) {
	dir := t.TempDir()
	store1 := NewTraceStore(dir)

	now := time.Now().UTC()
	store1.Store(StoredSpan{
		TraceID: "persist-test", SpanID: "s1", Name: "forge.run", Status: "ok",
		Service: "forge", Start: now, Duration: 50 * time.Millisecond,
	})

	// Verify file was written
	files, _ := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	if len(files) != 1 {
		t.Fatalf("expected 1 jsonl file, got %d", len(files))
	}

	// Create new store reading from same dir
	store2 := NewTraceStore(dir)
	summaries, _ := store2.ListTraces(ListOpts{})
	if len(summaries) != 1 || summaries[0].TraceID != "persist-test" {
		t.Fatalf("persistence: expected persist-test, got %v", summaries)
	}
}

func TestFormatTrace(t *testing.T) {
	now := time.Now().UTC()
	detail := &TraceDetail{
		TraceID: "fmt-test",
		Summary: TraceSummary{
			TraceID: "fmt-test", RootSpan: "forge.run", SpanCount: 2,
			Duration: 100 * time.Millisecond, Status: "ok", Service: "forge",
		},
		Spans: []StoredSpan{
			{TraceID: "fmt-test", SpanID: "s1", Name: "forge.run", Status: "ok", Service: "forge", Start: now, Duration: 100 * time.Millisecond},
			{TraceID: "fmt-test", SpanID: "s2", ParentID: "s1", Name: "forge.build", Status: "error", Service: "forge", Start: now, Duration: 50 * time.Millisecond},
		},
		Tree: SpanNode{
			Span: StoredSpan{Name: "forge.run", Status: "ok", Duration: 100 * time.Millisecond},
			Children: []SpanNode{
				{Span: StoredSpan{Name: "forge.build", Status: "error", Duration: 50 * time.Millisecond}},
			},
		},
	}

	output := FormatTrace(detail)
	if len(output) == 0 {
		t.Fatal("empty FormatTrace output")
	}
	_ = output // just verify it doesn't panic
}

func TestExportToFile(t *testing.T) {
	dir := t.TempDir()
	store := NewTraceStore(dir)

	now := time.Now().UTC()
	store.Store(StoredSpan{
		TraceID: "export-test", SpanID: "s1", Name: "forge.run", Status: "ok",
		Service: "forge", Start: now, End: now.Add(50 * time.Millisecond), Duration: 50 * time.Millisecond,
	})

	// Jaeger export to file
	jtraces, _ := store.ExportJaeger(ListOpts{})
	data, _ := json.MarshalIndent(jtraces, "", "  ")
	outPath := filepath.Join(t.TempDir(), "jaeger.json")
	os.WriteFile(outPath, data, 0o644)

	var parsed []JaegerTrace
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Jaeger round-trip: %v", err)
	}

	// Zipkin export to file
	zspans, _ := store.ExportZipkin(ListOpts{})
	data, _ = json.MarshalIndent(zspans, "", "  ")
	if len(data) == 0 {
		t.Fatal("empty Zipkin export")
	}

	// OTLP export
	otlp, _ := store.ExportOpenTelemetry(ListOpts{})
	data, _ = json.MarshalIndent(otlp, "", "  ")
	if len(data) == 0 {
		t.Fatal("empty OTLP export")
	}
}
