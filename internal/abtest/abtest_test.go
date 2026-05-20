package abtest

import (
	"testing"
)

func TestCreateExperiment(t *testing.T) {
	s := NewStore(t.TempDir())
	exp, err := s.Create("test", "prompt",
		[]Variant{{Name: "A", Model: "m1"}, {Name: "B", Model: "m2"}}, 30)
	if err != nil {
		t.Fatal(err)
	}
	if exp.Status != "draft" {
		t.Errorf("expected draft, got %s", exp.Status)
	}
	if len(exp.Variants) != 2 {
		t.Errorf("expected 2 variants, got %d", len(exp.Variants))
	}
}

func TestStartAndRecord(t *testing.T) {
	s := NewStore(t.TempDir())
	exp, _ := s.Create("test", "prompt",
		[]Variant{{Name: "A", Model: "m1"}, {Name: "B", Model: "m2"}}, 2)
	exp, err := s.Start(exp.ID)
	if err != nil {
		t.Fatal(err)
	}
	if exp.Status != "running" {
		t.Fatalf("expected running, got %s", exp.Status)
	}
	// Record 2 results for each variant to trigger auto-complete
	for i := 0; i < 2; i++ {
		_, err = s.RecordResult(exp.ID, Result{Variant: "A", Score: 0.8 + float64(i)*0.05, Success: true})
		if err != nil {
			t.Fatalf("RecordResult A: %v", err)
		}
		_, err = s.RecordResult(exp.ID, Result{Variant: "B", Score: 0.7 + float64(i)*0.05, Success: true})
		if err != nil {
			t.Fatalf("RecordResult B: %v", err)
		}
	}
	got, _ := s.Get(exp.ID)
	if got.Status != "completed" {
		t.Errorf("expected completed, got %s", got.Status)
	}
}

func TestRecordInvalidVariant(t *testing.T) {
	s := NewStore(t.TempDir())
	exp, _ := s.Create("test", "prompt", []Variant{{Name: "A", Model: "m1"}}, 10)
	s.Start(exp.ID)
	_, err := s.RecordResult(exp.ID, Result{Variant: "C", Score: 0.5})
	if err == nil {
		t.Error("expected error for invalid variant")
	}
}

func TestAnalyze(t *testing.T) {
	s := NewStore(t.TempDir())
	exp, _ := s.Create("test", "prompt",
		[]Variant{{Name: "A", Model: "gpt-4"}, {Name: "B", Model: "claude"}}, 5)
	s.Start(exp.ID)
	for i := 0; i < 6; i++ {
		s.RecordResult(exp.ID, Result{Variant: "A", Score: 0.70 + float64(i)*0.02, LatencyMS: 500, CostUSD: 0.08, Success: true})
		s.RecordResult(exp.ID, Result{Variant: "B", Score: 0.80 + float64(i)*0.02, LatencyMS: 400, CostUSD: 0.05, Success: true})
	}
	got, _ := s.Get(exp.ID)
	a := Analyze(got)
	if len(a.VariantStats) != 2 {
		t.Fatalf("expected 2 variant stats, got %d", len(a.VariantStats))
	}
	if a.Winner != "B" {
		t.Errorf("expected B to win, got %q", a.Winner)
	}
	if a.Recommendation == "" {
		t.Error("expected recommendation")
	}
}

func TestAnalyzeInsufficientData(t *testing.T) {
	exp := &Experiment{ID: "t", Name: "t", Variants: []Variant{{Name: "A", Model: "m1"}}, Results: []Result{}}
	a := Analyze(exp)
	if a.Winner != "" {
		t.Error("expected no winner with insufficient data")
	}
}

func TestFormatAnalysis(t *testing.T) {
	a := &Analysis{
		ExperimentID: "t", ExperimentName: "test", Winner: "B",
		Confidence: 0.85, Significant: true, Recommendation: "B wins.",
		VariantStats: []VariantStats{{Name: "A", N: 30, MeanScore: 0.75}, {Name: "B", N: 30, MeanScore: 0.85}},
	}
	if FormatAnalysis(a) == "" {
		t.Error("expected non-empty output")
	}
}

func TestGetNotFound(t *testing.T) {
	_, err := NewStore(t.TempDir()).Get("nope")
	if err == nil {
		t.Error("expected error")
	}
}

func TestList(t *testing.T) {
	s := NewStore(t.TempDir())
	s.Create("t1", "", []Variant{{Name: "A", Model: "m1"}}, 10)
	s.Create("t2", "", []Variant{{Name: "A", Model: "m1"}}, 10)
	exps, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(exps) != 2 {
		t.Errorf("expected 2, got %d", len(exps))
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	s1 := NewStore(dir)
	exp, _ := s1.Create("persist", "", []Variant{{Name: "A", Model: "m1"}}, 10)
	s2 := NewStore(dir)
	got, err := s2.Get(exp.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "persist" {
		t.Errorf("expected persist, got %s", got.Name)
	}
}

func TestComplete(t *testing.T) {
	s := NewStore(t.TempDir())
	exp, _ := s.Create("t", "", []Variant{{Name: "A", Model: "m1"}}, 10)
	got, err := s.Complete(exp.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "completed" {
		t.Errorf("expected completed, got %s", got.Status)
	}
}

func TestCancel(t *testing.T) {
	s := NewStore(t.TempDir())
	exp, _ := s.Create("t", "", []Variant{{Name: "A", Model: "m1"}}, 10)
	got, err := s.Cancel(exp.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "cancelled" {
		t.Errorf("expected cancelled, got %s", got.Status)
	}
}

func TestStartNonDraft(t *testing.T) {
	s := NewStore(t.TempDir())
	exp, _ := s.Create("t", "", []Variant{{Name: "A", Model: "m1"}}, 10)
	s.Start(exp.ID)
	_, err := s.Start(exp.ID)
	if err == nil {
		t.Error("expected error starting non-draft")
	}
}

func TestRecordOnDraft(t *testing.T) {
	s := NewStore(t.TempDir())
	exp, _ := s.Create("t", "", []Variant{{Name: "A", Model: "m1"}}, 10)
	_, err := s.RecordResult(exp.ID, Result{Variant: "A", Score: 0.5})
	if err == nil {
		t.Error("expected error recording on draft")
	}
}
