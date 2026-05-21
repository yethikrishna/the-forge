package breed_test

import (
	"testing"

	"github.com/forge/sword/internal/optimize/breed"
)

func TestInitialize(t *testing.T) {
	traits := []breed.Trait{
		{Name: "model", Type: "enum", Values: []string{"sonnet", "opus", "gpt4o"}},
		{Name: "temperature", Type: "float", Min: 0.0, Max: 1.0, Step: 0.1},
	}

	fitness := func(g breed.Genome) float64 {
		if g.Traits["model"] == "sonnet" {
			return 0.9
		}
		return 0.5
	}

	e := breed.NewEvolver(traits, fitness, "")
	e.PopulationSize = 10
	pop := e.Initialize()

	if len(pop) != 10 {
		t.Errorf("expected 10, got %d", len(pop))
	}
}

func TestEvolve(t *testing.T) {
	traits := []breed.Trait{
		{Name: "model", Type: "enum", Values: []string{"sonnet", "opus", "gpt4o"}},
		{Name: "temperature", Type: "float", Min: 0.0, Max: 1.0, Step: 0.1},
	}

	fitness := func(g breed.Genome) float64 {
		if g.Traits["model"] == "sonnet" {
			return 0.9
		}
		return 0.5
	}

	e := breed.NewEvolver(traits, fitness, "")
	e.PopulationSize = 10
	e.Initialize()

	newPop := e.Evolve()
	if len(newPop) != 10 {
		t.Errorf("expected 10, got %d", len(newPop))
	}
}

func TestBest(t *testing.T) {
	traits := []breed.Trait{
		{Name: "model", Type: "enum", Values: []string{"sonnet", "opus"}},
	}

	fitness := func(g breed.Genome) float64 {
		if g.Traits["model"] == "sonnet" {
			return 0.95
		}
		return 0.5
	}

	e := breed.NewEvolver(traits, fitness, "")
	e.PopulationSize = 20
	e.Initialize()

	// Run multiple generations
	for i := 0; i < 10; i++ {
		e.Evolve()
	}

	best := e.Best()
	if best == nil {
		t.Fatal("should have a best genome")
	}
	if best.Fitness < 0.8 {
		t.Errorf("best fitness should converge higher, got %.4f", best.Fitness)
	}
}

func TestBestFitness(t *testing.T) {
	traits := []breed.Trait{
		{Name: "x", Type: "enum", Values: []string{"a", "b", "c"}},
	}

	fitness := func(g breed.Genome) float64 {
		if g.Traits["x"] == "a" {
			return 1.0
		}
		return 0.1
	}

	e := breed.NewEvolver(traits, fitness, "")
	e.PopulationSize = 20
	e.Initialize()

	bf := e.BestFitness()
	if bf <= 0 {
		t.Error("best fitness should be positive")
	}
}

func TestAverageFitness(t *testing.T) {
	traits := []breed.Trait{
		{Name: "x", Type: "enum", Values: []string{"a", "b"}},
	}

	fitness := func(g breed.Genome) float64 {
		return 0.5
	}

	e := breed.NewEvolver(traits, fitness, "")
	e.PopulationSize = 10
	e.Initialize()

	af := e.AverageFitness()
	if af != 0.5 {
		t.Errorf("expected 0.5, got %.4f", af)
	}
}

func TestDiversity(t *testing.T) {
	traits := []breed.Trait{
		{Name: "model", Type: "enum", Values: []string{"sonnet", "opus", "gpt4o", "flash"}},
	}

	fitness := func(g breed.Genome) float64 {
		return 0.5
	}

	e := breed.NewEvolver(traits, fitness, "")
	e.PopulationSize = 20
	e.Initialize()

	div := e.Diversity()
	if div < 0 || div > 1 {
		t.Errorf("diversity should be 0-1, got %.4f", div)
	}
}

func TestRecordRun(t *testing.T) {
	traits := []breed.Trait{
		{Name: "x", Type: "enum", Values: []string{"a", "b"}},
	}

	fitness := func(g breed.Genome) float64 {
		return 0.5
	}

	e := breed.NewEvolver(traits, fitness, "")
	e.PopulationSize = 5
	pop := e.Initialize()

	e.RecordRun(pop[0].ID, 0.9)
}

func TestDefaultAgentTraits(t *testing.T) {
	traits := breed.DefaultAgentTraits()

	if len(traits) == 0 {
		t.Error("should have default traits")
	}

	// Check model trait
	var hasModel bool
	for _, t := range traits {
		if t.Name == "model" {
			hasModel = true
			if len(t.Values) == 0 {
				// model has values
			}
		}
	}
	if !hasModel {
		t.Error("should have model trait")
	}
}

func TestFormatGeneration(t *testing.T) {
	pop := []breed.Genome{
		{ID: "gen0-0", Traits: map[string]string{"model": "sonnet"}, Fitness: 0.95, Runs: 3},
		{ID: "gen0-1", Traits: map[string]string{"model": "opus"}, Fitness: 0.80, Runs: 2},
	}

	formatted := breed.FormatGeneration(pop, 0)
	if formatted == "" {
		t.Error("formatted output should not be empty")
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/breed.json"

	traits := []breed.Trait{
		{Name: "x", Type: "enum", Values: []string{"a", "b"}},
	}

	fitness := func(g breed.Genome) float64 {
		return 0.5
	}

	e := breed.NewEvolver(traits, fitness, path)
	e.PopulationSize = 5
	e.Initialize()

	e2 := breed.NewEvolver(traits, fitness, path)
	if len(e2.Population()) != 5 {
		t.Errorf("expected 5 after reload, got %d", len(e2.Population()))
	}
}
