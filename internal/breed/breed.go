// Package breed provides agent evolution through genetic optimization.
// The forge learns which blades cut truest, and forges them sharper.
package breed

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Trait represents an evolvable agent trait.
type Trait struct {
	Name     string   `json:"name"`
	Type     string   `json:"type"`     // string, float, int, enum
	Values   []string `json:"values"`   // for enum type
	Min      float64  `json:"min"`      // for float/int type
	Max      float64  `json:"max"`      // for float/int type
	Step     float64  `json:"step"`     // for float type
	Default  string   `json:"default"`
}

// Genome is a set of trait values for an agent.
type Genome struct {
	ID       string            `json:"id"`
	Traits   map[string]string `json:"traits"`
	Fitness  float64           `json:"fitness"`
	Runs     int               `json:"runs"`
	Parent1  string            `json:"parent1,omitempty"`
	Parent2  string            `json:"parent2,omitempty"`
	Generation int             `json:"generation"`
	CreatedAt time.Time        `json:"created_at"`
}

// Population is a collection of genomes.
type Population struct {
	Traits      []Genome `json:"traits"`
	Generation  int      `json:"generation"`
	BestFitness float64  `json:"best_fitness"`
}

// FitnessFunc evaluates a genome's fitness.
type FitnessFunc func(genome Genome) float64

// Evolver evolves agent configurations using genetic algorithms.
type Evolver struct {
	traits     []Trait
	population []Genome
	fitness    FitnessFunc
	store      string
	mu         sync.Mutex
	rng        *rand.Rand
	bestFit    float64

	// Parameters
	PopulationSize int     `json:"population_size"`
	MutationRate   float64 `json:"mutation_rate"`
	CrossoverRate  float64 `json:"crossover_rate"`
	Elitism        int     `json:"elitism"` // keep top N unchanged
}

// NewEvolver creates a new genetic evolver.
func NewEvolver(traits []Trait, fitness FitnessFunc, storePath string) *Evolver {
	e := &Evolver{
		traits:         traits,
		fitness:        fitness,
		store:          storePath,
		rng:            rand.New(rand.NewSource(time.Now().UnixNano())),
		PopulationSize: 20,
		MutationRate:   0.15,
		CrossoverRate:  0.7,
		Elitism:        2,
	}
	e.load()
	return e
}

// Initialize creates the initial random population.
func (e *Evolver) Initialize() []Genome {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.population = make([]Genome, e.PopulationSize)

	for i := 0; i < e.PopulationSize; i++ {
		genome := Genome{
			ID:         fmt.Sprintf("gen%d-%d", 0, i),
			Traits:     e.randomTraits(),
			Generation: 0,
			CreatedAt:  time.Now().UTC(),
		}
		genome.Fitness = e.fitness(genome)
		genome.Runs = 1
		e.population[i] = genome
	}

	e.sortPopulation()
	e.save()

	return e.population
}

// Evolve runs one generation of evolution.
func (e *Evolver) Evolve() []Genome {
	e.mu.Lock()
	defer e.mu.Unlock()

	if len(e.population) == 0 {
		return nil
	}

	gen := e.population[0].Generation + 1
	var newPop []Genome

	// Elitism: keep top N
	for i := 0; i < e.Elitism && i < len(e.population); i++ {
		elite := e.population[i]
		elite.ID = fmt.Sprintf("gen%d-elite%d", gen, i)
		newPop = append(newPop, elite)
	}

	// Generate rest through crossover and mutation
	for len(newPop) < e.PopulationSize {
		p1 := e.tournamentSelect()
		p2 := e.tournamentSelect()

		var child Genome
		if e.rng.Float64() < e.CrossoverRate {
			child = e.crossover(p1, p2, gen)
		} else {
			child = Genome{
				ID:         fmt.Sprintf("gen%d-%d", gen, len(newPop)),
				Traits:     copyMap(p1.Traits),
				Generation: gen,
				Parent1:    p1.ID,
				CreatedAt:  time.Now().UTC(),
			}
		}

		// Mutation
		if e.rng.Float64() < e.MutationRate {
			e.mutate(&child)
		}

		child.Fitness = e.fitness(child)
		child.Runs = 1
		newPop = append(newPop, child)
	}

	e.population = newPop
	e.sortPopulation()

	// Track best fitness
	if len(e.population) > 0 {
		e.bestFit = e.population[0].Fitness
	}

	e.save()
	return e.population
}

// Best returns the best genome in the population.
func (e *Evolver) Best() *Genome {
	e.mu.Lock()
	defer e.mu.Unlock()

	if len(e.population) == 0 {
		return nil
	}
	best := e.population[0]
	return &best
}

// BestFitness returns the highest fitness in the population.
func (e *Evolver) BestFitness() float64 {
	e.mu.Lock()
	defer e.mu.Unlock()

	if len(e.population) == 0 {
		return 0
	}
	return e.population[0].Fitness
}

// AverageFitness returns the average fitness.
func (e *Evolver) AverageFitness() float64 {
	e.mu.Lock()
	defer e.mu.Unlock()

	if len(e.population) == 0 {
		return 0
	}

	var total float64
	for _, g := range e.population {
		total += g.Fitness
	}
	return total / float64(len(e.population))
}

// Diversity returns the genetic diversity (0-1).
func (e *Evolver) Diversity() float64 {
	e.mu.Lock()
	defer e.mu.Unlock()

	if len(e.population) < 2 {
		return 0
	}

	var differences int
	var comparisons int

	for i := 0; i < len(e.population)-1; i++ {
		for j := i + 1; j < len(e.population); j++ {
			for k, v := range e.population[i].Traits {
				comparisons++
				if e.population[j].Traits[k] != v {
					differences++
				}
			}
		}
	}

	if comparisons == 0 {
		return 0
	}
	return float64(differences) / float64(comparisons)
}

// Population returns the current population.
func (e *Evolver) Population() []Genome {
	e.mu.Lock()
	defer e.mu.Unlock()

	out := make([]Genome, len(e.population))
	copy(out, e.population)
	return out
}

// RecordRun updates a genome's fitness with a new run result.
func (e *Evolver) RecordRun(genomeID string, fitness float64) {
	e.mu.Lock()
	defer e.mu.Unlock()

	for i := range e.population {
		if e.population[i].ID == genomeID {
			// Running average
			n := float64(e.population[i].Runs)
			e.population[i].Fitness = (e.population[i].Fitness*n + fitness) / (n + 1)
			e.population[i].Runs++
			break
		}
	}

	e.sortPopulation()
	e.save()
}

// randomTraits generates random trait values.
func (e *Evolver) randomTraits() map[string]string {
	traits := make(map[string]string)

	for _, t := range e.traits {
		switch t.Type {
		case "enum":
			if len(t.Values) > 0 {
				traits[t.Name] = t.Values[e.rng.Intn(len(t.Values))]
			}
		case "float":
			v := t.Min + e.rng.Float64()*(t.Max-t.Min)
			if t.Step > 0 {
				steps := math.Round((v - t.Min) / t.Step)
				v = t.Min + steps*t.Step
			}
			traits[t.Name] = fmt.Sprintf("%.4f", v)
		case "int":
			v := int(t.Min) + e.rng.Intn(int(t.Max-t.Min)+1)
			traits[t.Name] = fmt.Sprintf("%d", v)
		case "string":
			traits[t.Name] = t.Default
		}
	}

	return traits
}

// tournamentSelect selects a genome via tournament selection.
func (e *Evolver) tournamentSelect() Genome {
	const tournamentSize = 3

	best := e.population[e.rng.Intn(len(e.population))]
	for i := 1; i < tournamentSize; i++ {
		candidate := e.population[e.rng.Intn(len(e.population))]
		if candidate.Fitness > best.Fitness {
			best = candidate
		}
	}
	return best
}

// crossover creates a child from two parents.
func (e *Evolver) crossover(p1, p2 Genome, gen int) Genome {
	child := Genome{
		ID:         fmt.Sprintf("gen%d-%d", gen, e.rng.Intn(100000)),
		Traits:     make(map[string]string),
		Generation: gen,
		Parent1:    p1.ID,
		Parent2:    p2.ID,
		CreatedAt:  time.Now().UTC(),
	}

	for _, t := range e.traits {
		if e.rng.Float64() < 0.5 {
			child.Traits[t.Name] = p1.Traits[t.Name]
		} else {
			child.Traits[t.Name] = p2.Traits[t.Name]
		}
	}

	return child
}

// mutate randomly modifies one trait.
func (e *Evolver) mutate(g *Genome) {
	if len(e.traits) == 0 {
		return
	}

	trait := e.traits[e.rng.Intn(len(e.traits))]

	switch trait.Type {
	case "enum":
		if len(trait.Values) > 0 {
			g.Traits[trait.Name] = trait.Values[e.rng.Intn(len(trait.Values))]
		}
	case "float":
		v := trait.Min + e.rng.Float64()*(trait.Max-trait.Min)
		if trait.Step > 0 {
			steps := math.Round((v - trait.Min) / trait.Step)
			v = trait.Min + steps*trait.Step
		}
		g.Traits[trait.Name] = fmt.Sprintf("%.4f", v)
	case "int":
		v := int(trait.Min) + e.rng.Intn(int(trait.Max-trait.Min)+1)
		g.Traits[trait.Name] = fmt.Sprintf("%d", v)
	}
}

func (e *Evolver) sortPopulation() {
	sort.Slice(e.population, func(i, j int) bool {
		return e.population[i].Fitness > e.population[j].Fitness
	})
}

func copyMap(m map[string]string) map[string]string {
	c := make(map[string]string, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}

func (e *Evolver) load() {
	if e.store == "" {
		return
	}
	data, err := os.ReadFile(e.store)
	if err != nil {
		return
	}
	json.Unmarshal(data, &e.population)
}

func (e *Evolver) save() {
	if e.store == "" {
		return
	}
	data, err := json.MarshalIndent(e.population, "", "  ")
	if err != nil {
		return
	}
	dir := filepath.Dir(e.store)
	os.MkdirAll(dir, 0o755)
	os.WriteFile(e.store, data, 0o644)
}

// FormatGeneration formats a generation for display.
func FormatGeneration(pop []Genome, gen int) string {
	var b string
	b += fmt.Sprintf("Generation %d — Top Genomes\n", gen)
	b += fmt.Sprintf("%s\n", strings.Repeat("—", 50))

	limit := 5
	if len(pop) < limit {
		limit = len(pop)
	}

	for i := 0; i < limit; i++ {
		g := pop[i]
		b += fmt.Sprintf("  #%d %s (fitness: %.4f, runs: %d)\n", i+1, g.ID, g.Fitness, g.Runs)
		for k, v := range g.Traits {
			b += fmt.Sprintf("     %-15s %s\n", k, v)
		}
	}

	return b
}

// DefaultAgentTraits returns the default evolvable traits for agents.
func DefaultAgentTraits() []Trait {
	return []Trait{
		{Name: "model", Type: "enum", Values: []string{
			"anthropic/claude-sonnet-4-20250514",
			"anthropic/claude-opus-4-20250514",
			"openai/gpt-4o",
			"openai/gpt-5-mini",
			"google/gemini-2.5-pro",
			"google/gemini-2.5-flash",
			"deepseek/deepseek-v3",
		}},
		{Name: "temperature", Type: "float", Min: 0.0, Max: 1.0, Step: 0.1},
		{Name: "max_tokens", Type: "int", Min: 256, Max: 8192},
		{Name: "jail", Type: "enum", Values: []string{"strict", "moderate", "none"}},
		{Name: "retry_count", Type: "int", Min: 0, Max: 5},
	}
}
