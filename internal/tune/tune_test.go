package tune

import (
	"math"
	"testing"
)

func TestCreateStudy(t *testing.T) {
	o := NewOptimizer(t.TempDir())
	params := DefaultAgentParams()
	study := o.CreateStudy("test-study", params, "maximize")

	if study.Name != "test-study" {
		t.Errorf("expected test-study, got %s", study.Name)
	}
	if study.Direction != "maximize" {
		t.Errorf("expected maximize, got %s", study.Direction)
	}
	if len(study.Params) != 4 {
		t.Errorf("expected 4 params, got %d", len(study.Params))
	}
}

func TestSuggest(t *testing.T) {
	o := NewOptimizer(t.TempDir())
	params := DefaultAgentParams()
	o.CreateStudy("test-suggest", params, "maximize")

	values, err := o.Suggest()
	if err != nil {
		t.Fatal(err)
	}

	if len(values) != 4 {
		t.Errorf("expected 4 param values, got %d", len(values))
	}

	// Check temperature is within range
	temp, ok := values["temperature"].(float64)
	if !ok {
		t.Error("expected temperature to be float64")
	}
	if temp < 0 || temp > 2.0 {
		t.Errorf("temperature out of range: %f", temp)
	}
}

func TestRecordTrial(t *testing.T) {
	o := NewOptimizer(t.TempDir())
	params := DefaultAgentParams()
	o.CreateStudy("test-record", params, "maximize")

	values, _ := o.Suggest()
	o.RecordTrial(values, 0.85, 1.5, "")

	best := o.Best()
	if best == nil {
		t.Fatal("expected best trial")
	}
	if best.Score != 0.85 {
		t.Errorf("expected score 0.85, got %f", best.Score)
	}
}

func TestRecordFailedTrial(t *testing.T) {
	o := NewOptimizer(t.TempDir())
	params := DefaultAgentParams()
	o.CreateStudy("test-failed", params, "maximize")

	values, _ := o.Suggest()
	o.RecordTrial(values, 0, 0, "timeout")

	best := o.Best()
	// Failed trial should not be best
	if best != nil && best.Score == 0 {
		t.Error("failed trial should not be best")
	}
}

func TestMinimize(t *testing.T) {
	o := NewOptimizer(t.TempDir())
	params := []ParamDef{
		{Name: "lr", Type: ParamFloat, Min: 0.001, Max: 1.0},
	}
	o.CreateStudy("test-min", params, "minimize")

	o.RecordTrial(ParamValues{"lr": 0.1}, 0.5, 1.0, "")
	o.RecordTrial(ParamValues{"lr": 0.01}, 0.3, 1.0, "")

	best := o.Best()
	if best == nil {
		t.Fatal("expected best trial")
	}
	if best.Score != 0.3 {
		t.Errorf("expected best score 0.3, got %f", best.Score)
	}
}

func TestMultipleTrials(t *testing.T) {
	o := NewOptimizer(t.TempDir())
	params := DefaultAgentParams()
	o.CreateStudy("test-multi", params, "maximize")

	for i := 0; i < 10; i++ {
		values, _ := o.Suggest()
		score := 0.5 + o.rng.Float64()*0.5
		o.RecordTrial(values, score, 1.0, "")
	}

	study := o.Study()
	if len(study.Trials) != 10 {
		t.Errorf("expected 10 trials, got %d", len(study.Trials))
	}
	if study.BestScore <= 0.5 {
		t.Error("expected best score > 0.5")
	}
}

func TestHistory(t *testing.T) {
	o := NewOptimizer(t.TempDir())
	params := DefaultAgentParams()
	o.CreateStudy("test-history", params, "maximize")

	for i := 0; i < 5; i++ {
		values, _ := o.Suggest()
		o.RecordTrial(values, float64(i)/5, 1.0, "")
	}

	history := o.History()
	if len(history) != 5 {
		t.Errorf("expected 5 trials, got %d", len(history))
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	o := NewOptimizer(dir)
	params := DefaultAgentParams()
	o.CreateStudy("test-persist", params, "maximize")

	values, _ := o.Suggest()
	o.RecordTrial(values, 0.9, 2.0, "")

	if err := o.Save(); err != nil {
		t.Fatal(err)
	}

	o2 := NewOptimizer(dir)
	if err := o2.Load("test-persist"); err != nil {
		t.Fatal(err)
	}

	study := o2.Study()
	if study.Name != "test-persist" {
		t.Errorf("expected test-persist, got %s", study.Name)
	}
	if len(study.Trials) != 1 {
		t.Errorf("expected 1 trial, got %d", len(study.Trials))
	}
}

func TestDefaultAgentParams(t *testing.T) {
	params := DefaultAgentParams()
	if len(params) != 4 {
		t.Errorf("expected 4 default params, got %d", len(params))
	}
}

func TestFormatTrial(t *testing.T) {
	trial := Trial{
		ID:     1,
		Params: ParamValues{"temperature": 0.7},
		Score:  0.92,
		Status: "completed",
	}
	output := FormatTrial(trial)
	if output == "" {
		t.Error("expected non-empty output")
	}
}

func TestFormatStudy(t *testing.T) {
	study := &Study{
		Name:      "test",
		Direction: "maximize",
		Trials:    make([]Trial, 5),
		BestScore: 0.95,
	}
	output := FormatStudy(study)
	if output == "" {
		t.Error("expected non-empty output")
	}
}

func TestEuclideanDist(t *testing.T) {
	o := NewOptimizer(t.TempDir())
	a := []float64{0.0, 0.0}
	b := []float64{3.0, 4.0}
	dist := o.euclideanDist(a, b)
	if math.Abs(dist-5.0) > 0.01 {
		t.Errorf("expected 5.0, got %f", dist)
	}
}

func TestParamsToVector(t *testing.T) {
	o := NewOptimizer(t.TempDir())
	params := []ParamDef{
		{Name: "temp", Type: ParamFloat, Min: 0, Max: 2},
		{Name: "tokens", Type: ParamInt, Min: 0, Max: 1000},
	}
	o.CreateStudy("test-vec", params, "maximize")

	vector := o.paramsToVector(ParamValues{"temp": 1.0, "tokens": 500})
	if len(vector) != 2 {
		t.Errorf("expected 2D vector, got %d", len(vector))
	}
	if math.Abs(vector[0]-0.5) > 0.01 {
		t.Errorf("expected 0.5 for temp, got %f", vector[0])
	}
}
