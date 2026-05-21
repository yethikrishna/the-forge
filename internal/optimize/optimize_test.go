package optimize_test

import (
	"testing"

	"github.com/forge/sword/internal/optimize/breed"
	"github.com/forge/sword/internal/optimize/dream"
	"github.com/forge/sword/internal/optimize/tune"
)

func TestDreamStore(t *testing.T) {
	store := dream.NewStore(t.TempDir())
	if store == nil {
		t.Fatal("NewStore should return a store")
	}
}

func TestDreamSession(t *testing.T) {
	store := dream.NewStore(t.TempDir())
	session := dream.NewDreamSession(store)
	if session == nil {
		t.Fatal("NewDreamSession should return a session")
	}

	session.LoadSessions([]dream.Session{
		{ID: "s1", Success: true, Duration: 30, TokensUsed: 1000},
		{ID: "s2", Success: true, Duration: 45, TokensUsed: 1500},
		{ID: "s3", Success: false, Duration: 10, TokensUsed: 500},
	})

	report, err := session.Run()
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if report == nil {
		t.Fatal("Run should return a report")
	}
}

func TestDreamSaveReport(t *testing.T) {
	store := dream.NewStore(t.TempDir())
	report := &dream.DreamReport{
		FilesIndexed: 5,
	}
	if err := store.SaveReport(report); err != nil {
		t.Fatalf("SaveReport error: %v", err)
	}
}

func TestBreedEvolver(t *testing.T) {
	traits := []breed.Trait{
		{Name: "model", Values: []string{"gpt-4", "claude", "llama"}},
		{Name: "temperature", Values: []string{"0.3", "0.7", "1.0"}},
	}
	evolver := breed.NewEvolver(traits, breed.FitnessFunc(func(g breed.Genome) float64 {
		return 0.5
	}), t.TempDir())

	population := evolver.Initialize()
	if len(population) == 0 {
		t.Error("Initialize should return a population")
	}
}

func TestBreedEvolve(t *testing.T) {
	traits := []breed.Trait{
		{Name: "model", Values: []string{"a", "b"}},
	}
	evolver := breed.NewEvolver(traits, breed.FitnessFunc(func(g breed.Genome) float64 {
		return 0.5
	}), t.TempDir())
	evolver.Initialize()

	nextGen := evolver.Evolve()
	if len(nextGen) == 0 {
		t.Error("Evolve should return next generation")
	}
}

func TestBreedRecordRun(t *testing.T) {
	traits := []breed.Trait{
		{Name: "model", Values: []string{"a", "b"}},
	}
	evolver := breed.NewEvolver(traits, breed.FitnessFunc(func(g breed.Genome) float64 { return 0.5 }), t.TempDir())
	population := evolver.Initialize()

	evolver.RecordRun(population[0].ID, 0.85)
}

func TestBreedDiversity(t *testing.T) {
	traits := []breed.Trait{
		{Name: "model", Values: []string{"a", "b", "c"}},
	}
	evolver := breed.NewEvolver(traits, breed.FitnessFunc(func(g breed.Genome) float64 { return 0.5 }), t.TempDir())
	evolver.Initialize()

	div := evolver.Diversity()
	if div < 0 || div > 1 {
		t.Errorf("Diversity = %f, want [0,1]", div)
	}
}

func TestBreedBest(t *testing.T) {
	traits := []breed.Trait{
		{Name: "model", Values: []string{"a", "b"}},
	}
	evolver := breed.NewEvolver(traits, breed.FitnessFunc(func(g breed.Genome) float64 {
		return 0.8
	}), t.TempDir())
	evolver.Initialize()

	best := evolver.Best()
	_ = best
}

func TestTuneOptimizer(t *testing.T) {
	optimizer := tune.NewOptimizer(t.TempDir())
	if optimizer == nil {
		t.Fatal("NewOptimizer should return an optimizer")
	}
}

func TestTuneCreateStudy(t *testing.T) {
	optimizer := tune.NewOptimizer(t.TempDir())

	study := optimizer.CreateStudy("test-study", tune.DefaultAgentParams(), "maximize")
	if study == nil {
		t.Fatal("CreateStudy should return a study")
	}
	if study.Name != "test-study" {
		t.Errorf("Study name = %q, want %q", study.Name, "test-study")
	}
}

func TestTuneSuggestAndRecord(t *testing.T) {
	optimizer := tune.NewOptimizer(t.TempDir())
	optimizer.CreateStudy("test", tune.DefaultAgentParams(), "maximize")

	params, err := optimizer.Suggest()
	if err != nil {
		t.Fatalf("Suggest error: %v", err)
	}
	if len(params) == 0 {
		t.Error("Suggest should return parameters")
	}

	optimizer.RecordTrial(params, 0.85, 2.5, "")
}

func TestTuneBest(t *testing.T) {
	optimizer := tune.NewOptimizer(t.TempDir())
	optimizer.CreateStudy("test", tune.DefaultAgentParams(), "maximize")

	p1, _ := optimizer.Suggest()
	optimizer.RecordTrial(p1, 0.7, 1.0, "")

	p2, _ := optimizer.Suggest()
	optimizer.RecordTrial(p2, 0.9, 1.5, "")

	best := optimizer.Best()
	if best == nil {
		t.Fatal("Best should return a trial")
	}
	if best.Score < 0 {
		t.Error("Best score should be non-negative")
	}
}

func TestTuneHistory(t *testing.T) {
	optimizer := tune.NewOptimizer(t.TempDir())
	optimizer.CreateStudy("test", tune.DefaultAgentParams(), "maximize")

	p, _ := optimizer.Suggest()
	optimizer.RecordTrial(p, 0.5, 1.0, "")
	p, _ = optimizer.Suggest()
	optimizer.RecordTrial(p, 0.6, 1.0, "")

	history := optimizer.History()
	if len(history) < 2 {
		t.Errorf("History = %d trials, want at least 2", len(history))
	}
}

func TestTuneSaveLoad(t *testing.T) {
	dir := t.TempDir()
	optimizer := tune.NewOptimizer(dir)
	optimizer.CreateStudy("persist-test", tune.DefaultAgentParams(), "maximize")

	p, _ := optimizer.Suggest()
	optimizer.RecordTrial(p, 0.75, 1.0, "")

	if err := optimizer.Save(); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	optimizer2 := tune.NewOptimizer(dir)
	if err := optimizer2.Load("persist-test"); err != nil {
		t.Fatalf("Load error: %v", err)
	}
}

func TestDefaultAgentParams(t *testing.T) {
	params := tune.DefaultAgentParams()
	if len(params) == 0 {
		t.Error("DefaultAgentParams should return parameters")
	}
}
