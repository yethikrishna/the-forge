package bench

import (
	"strings"
	"testing"
)

func TestCreateBenchmark(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	b, err := store.CreateBenchmark("latency-test", "claude-sonnet-4", "coder", "fix bug", 10)
	if err != nil {
		t.Fatalf("CreateBenchmark: %v", err)
	}
	if b.Name != "latency-test" {
		t.Errorf("name: %s", b.Name)
	}
	if b.Iterations != 10 {
		t.Errorf("iterations: %d", b.Iterations)
	}
}

func TestGetBenchmark(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	created, _ := store.CreateBenchmark("test", "gpt-4", "coder", "task", 5)
	found, err := store.GetBenchmark(created.ID)
	if err != nil {
		t.Fatalf("GetBenchmark: %v", err)
	}
	if found.Name != "test" {
		t.Errorf("name: %s", found.Name)
	}
}

func TestListBenchmarks(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	store.CreateBenchmark("b1", "a", "x", "t", 5)
	store.CreateBenchmark("b2", "b", "y", "t", 5)
	list, err := store.ListBenchmarks()
	if err != nil {
		t.Fatalf("ListBenchmarks: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("count: %d", len(list))
	}
}

func TestDeleteBenchmark(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	b, _ := store.CreateBenchmark("test", "a", "x", "t", 5)
	if err := store.DeleteBenchmark(b.ID); err != nil {
		t.Fatalf("DeleteBenchmark: %v", err)
	}
	if _, err := store.GetBenchmark(b.ID); err == nil {
		t.Error("expected error after delete")
	}
}

func TestRecordMeasurement(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	b, _ := store.CreateBenchmark("test", "a", "x", "t", 5)
	m, err := store.RecordMeasurement(Measurement{
		BenchmarkID: b.ID,
		Iteration:   1,
		LatencyMS:   500,
		TokensIn:    100,
		TokensOut:   200,
		CostUSD:     0.05,
		Success:     true,
	})
	if err != nil {
		t.Fatalf("RecordMeasurement: %v", err)
	}
	if m.ID == "" {
		t.Error("expected auto-generated ID")
	}
}

func TestGetMeasurements(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	b, _ := store.CreateBenchmark("test", "a", "x", "t", 5)
	store.RecordMeasurement(Measurement{BenchmarkID: b.ID, Iteration: 1, LatencyMS: 500, Success: true})
	store.RecordMeasurement(Measurement{BenchmarkID: b.ID, Iteration: 2, LatencyMS: 600, Success: true})
	measurements, err := store.GetMeasurements(b.ID)
	if err != nil {
		t.Fatalf("GetMeasurements: %v", err)
	}
	if len(measurements) != 2 {
		t.Errorf("count: %d", len(measurements))
	}
}

func TestSummarize(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	b, _ := store.CreateBenchmark("test", "a", "x", "t", 5)
	store.RecordMeasurement(Measurement{BenchmarkID: b.ID, Iteration: 1, LatencyMS: 100, CostUSD: 0.01, Success: true, TokensIn: 50, TokensOut: 100})
	store.RecordMeasurement(Measurement{BenchmarkID: b.ID, Iteration: 2, LatencyMS: 200, CostUSD: 0.02, Success: true, TokensIn: 60, TokensOut: 120})
	store.RecordMeasurement(Measurement{BenchmarkID: b.ID, Iteration: 3, LatencyMS: 300, CostUSD: 0.03, Success: true, TokensIn: 70, TokensOut: 140})
	store.RecordMeasurement(Measurement{BenchmarkID: b.ID, Iteration: 4, LatencyMS: 400, CostUSD: 0.04, Success: true, TokensIn: 80, TokensOut: 160})
	store.RecordMeasurement(Measurement{BenchmarkID: b.ID, Iteration: 5, LatencyMS: 500, CostUSD: 0.05, Success: false, TokensIn: 90, TokensOut: 180})

	summary, err := store.Summarize(b.ID)
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if summary.TotalRuns != 5 {
		t.Errorf("total: %d", summary.TotalRuns)
	}
	if summary.SuccessRate != 0.8 {
		t.Errorf("success_rate: %.2f", summary.SuccessRate)
	}
	if summary.MeanLatencyMS != 300 {
		t.Errorf("mean_latency: %.1f", summary.MeanLatencyMS)
	}
	if summary.MinLatencyMS != 100 {
		t.Errorf("min: %d", summary.MinLatencyMS)
	}
	if summary.MaxLatencyMS != 500 {
		t.Errorf("max: %d", summary.MaxLatencyMS)
	}
}

func TestSummarizeEmpty(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	b, _ := store.CreateBenchmark("test", "a", "x", "t", 5)
	_, err := store.Summarize(b.ID)
	if err == nil {
		t.Error("expected error for no measurements")
	}
}

func TestCompareSummaries(t *testing.T) {
	a := &Summary{
		TotalRuns:     10,
		SuccessRate:   0.9,
		MeanLatencyMS: 500,
		P95LatencyMS:  800,
		MeanCostUSD:   0.05,
		ThroughputRPS: 2.0,
	}
	b := &Summary{
		TotalRuns:     10,
		SuccessRate:   0.95,
		MeanLatencyMS: 300,
		P95LatencyMS:  500,
		MeanCostUSD:   0.03,
		ThroughputRPS: 3.0,
	}
	c := CompareSummaries(a, b)
	if c.Better != "b" {
		t.Errorf("expected b to be better, got %s", c.Better)
	}
	if c.Delta["latency_mean"] >= 0 {
		t.Error("expected negative latency delta (b is faster)")
	}
}

func TestFormatSummary(t *testing.T) {
	s := &Summary{
		TotalRuns:     10,
		SuccessRate:   0.9,
		MeanLatencyMS: 500,
		P50LatencyMS:  450,
		P95LatencyMS:  800,
		P99LatencyMS:  900,
		MinLatencyMS:  300,
		MaxLatencyMS:  950,
		MeanCostUSD:   0.05,
	}
	out := FormatSummary(s)
	if !strings.Contains(out, "500.0ms") {
		t.Error("expected mean latency")
	}
	if !strings.Contains(out, "90.0%") {
		t.Error("expected success rate")
	}
}
