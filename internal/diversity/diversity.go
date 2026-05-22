// Package diversity ensures each organization carries unique cultural DNA,
// preventing homogenization across the forge ecosystem. No two orgs should
// ever be identical — diversity is measured, seeded, and evolved deliberately.
//
// This closes the gap of organizational monoculture: without intentional
// diversity tracking, orgs converge toward a single behavioral pattern,
// losing the resilience that comes from differentiated cultures.
package diversity

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"sync"
	"time"
)

// CultureGene represents a single heritable cultural trait.
type CultureGene struct {
	ID         string             `json:"id"`
	Name       string             `json:"name"`
	Category   string             `json:"category"`
	Expression string             `json:"expression"`
	Strength   float64            `json:"strength"`
	Traits     map[string]string  `json:"traits,omitempty"`
	MutatedAt  time.Time          `json:"mutated_at,omitempty"`
}

// CultureDNA is the full genetic makeup of an organization's culture.
type CultureDNA struct {
	OrgID       string        `json:"org_id"`
	Genes       []CultureGene `json:"genes"`
	Fingerprint string        `json:"fingerprint"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

// DiversityScore measures how distinct an org is from the population.
type DiversityScore struct {
	OrgID         string    `json:"org_id"`
	Score         float64   `json:"score"`
	PopulationAvg float64   `json:"population_avg"`
	Percentile    float64   `json:"percentile"`
	UniqueGenes   int       `json:"unique_genes"`
	CommonGenes   int       `json:"common_genes"`
	MeasuredAt    time.Time `json:"measured_at"`
}

// CultureSeed bootstraps a new org with distinct cultural traits.
type CultureSeed struct {
	ID           string            `json:"id"`
	SourceOrgID  string            `json:"source_org_id,omitempty"`
	Genome       []CultureGene     `json:"genome"`
	MutationRate float64           `json:"mutation_rate"`
	Constraints  map[string]string `json:"constraints,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
}

// Store provides thread-safe JSON file persistence for diversity data.
type Store struct {
	mu       sync.Mutex
	filePath string
	data     storeData
}

type storeData struct {
	DNAs   map[string]CultureDNA    `json:"dnas"`
	Scores map[string]DiversityScore `json:"scores"`
	Seeds  map[string]CultureSeed   `json:"seeds"`
}

// NewStore creates a Store backed by the given file path.
func NewStore(filePath string) *Store {
	return &Store{
		filePath: filePath,
		data: storeData{
			DNAs:   make(map[string]CultureDNA),
			Scores: make(map[string]DiversityScore),
			Seeds:  make(map[string]CultureSeed),
		},
	}
}

// Load reads persisted data from disk.
func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return json.Unmarshal(raw, &s.data)
}

// Save writes current data to disk.
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, raw, 0644)
}

// GenerateSeed creates a new CultureSeed, optionally inheriting from a source org
// with mutation.
func (s *Store) GenerateSeed(sourceOrgID string, mutationRate float64, constraints map[string]string) CultureSeed {
	s.mu.Lock()
	defer s.mu.Unlock()

	seed := CultureSeed{
		ID:           fmt.Sprintf("seed-%d", time.Now().UTC().UnixNano()),
		SourceOrgID:  sourceOrgID,
		MutationRate: mutationRate,
		Constraints:  constraints,
		CreatedAt:    time.Now().UTC(),
	}

	if sourceDNA, ok := s.data.DNAs[sourceOrgID]; ok {
		for _, gene := range sourceDNA.Genes {
			mutated := gene
			if rand.Float64() < mutationRate {
				mutated.Strength = clamp01(gene.Strength + rand.Float64()*0.3 - 0.15)
				mutated.MutatedAt = time.Now().UTC()
			}
			seed.Genome = append(seed.Genome, mutated)
		}
	}

	s.data.Seeds[seed.ID] = seed
	return seed
}

// MeasureDiversity computes a DiversityScore for the given org against the
// population of all stored orgs.
func (s *Store) MeasureDiversity(orgID string) (DiversityScore, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	dna, ok := s.data.DNAs[orgID]
	if !ok {
		return DiversityScore{}, nil
	}

	var allOther [][]CultureGene
	for id, other := range s.data.DNAs {
		if id != orgID {
			allOther = append(allOther, other.Genes)
		}
	}

	if len(allOther) == 0 {
		score := DiversityScore{
			OrgID:         orgID,
			Score:         1.0,
			PopulationAvg: 0,
			Percentile:    1.0,
			UniqueGenes:   len(dna.Genes),
			CommonGenes:   0,
			MeasuredAt:    time.Now().UTC(),
		}
		s.data.Scores[orgID] = score
		return score, nil
	}

	totalSim := 0.0
	for _, otherGenes := range allOther {
		totalSim += geneSetSimilarity(dna.Genes, otherGenes)
	}
	avgSim := totalSim / float64(len(allOther))
	diversityScore := 1.0 - avgSim

	unique, common := countUniqueGenes(dna.Genes, allOther)

	score := DiversityScore{
		OrgID:         orgID,
		Score:         diversityScore,
		PopulationAvg: 1.0 - avgSim,
		Percentile:    diversityScore,
		UniqueGenes:   unique,
		CommonGenes:   common,
		MeasuredAt:    time.Now().UTC(),
	}
	s.data.Scores[orgID] = score
	return score, nil
}

// CompareCultures returns a similarity score between two orgs (0.0–1.0).
func (s *Store) CompareCultures(orgA, orgB string) float64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	dnaA, okA := s.data.DNAs[orgA]
	dnaB, okB := s.data.DNAs[orgB]
	if !okA || !okB {
		return 0
	}
	return geneSetSimilarity(dnaA.Genes, dnaB.Genes)
}

// EvolveCulture applies a seed to an org, mutating its DNA over time.
func (s *Store) EvolveCulture(orgID, seedID string) (CultureDNA, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	dna, ok := s.data.DNAs[orgID]
	if !ok {
		dna = CultureDNA{
			OrgID:     orgID,
			CreatedAt: time.Now().UTC(),
		}
	}

	seed, seedOK := s.data.Seeds[seedID]
	if seedOK {
		existing := make(map[string]bool)
		for _, g := range dna.Genes {
			existing[g.ID] = true
		}
		for _, gene := range seed.Genome {
			if !existing[gene.ID] {
				dna.Genes = append(dna.Genes, gene)
			}
		}
	}

	dna.UpdatedAt = time.Now().UTC()
	dna.Fingerprint = computeFingerprint(dna.Genes)
	s.data.DNAs[orgID] = dna
	return dna, nil
}

// GenerateDiversityReport produces a summary of all org diversity scores.
func (s *Store) GenerateDiversityReport() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()

	scores := make([]DiversityScore, 0, len(s.data.Scores))
	for _, sc := range s.data.Scores {
		scores = append(scores, sc)
	}

	avg := 0.0
	min := 1.0
	max := 0.0
	for _, sc := range scores {
		avg += sc.Score
		if sc.Score < min {
			min = sc.Score
		}
		if sc.Score > max {
			max = sc.Score
		}
	}
	if len(scores) > 0 {
		avg /= float64(len(scores))
	}

	return map[string]interface{}{
		"org_count":     len(s.data.DNAs),
		"seed_count":    len(s.data.Seeds),
		"average_score": avg,
		"min_score":     min,
		"max_score":     max,
		"scores":        scores,
	}
}

// PutDNA stores a CultureDNA directly.
func (s *Store) PutDNA(dna CultureDNA) {
	s.mu.Lock()
	defer s.mu.Unlock()
	dna.UpdatedAt = time.Now().UTC()
	dna.Fingerprint = computeFingerprint(dna.Genes)
	s.data.DNAs[dna.OrgID] = dna
}

// GetDNA retrieves a CultureDNA by org ID.
func (s *Store) GetDNA(orgID string) (CultureDNA, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.data.DNAs[orgID]
	return d, ok
}

func geneSetSimilarity(a, b []CultureGene) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1.0
	}
	if len(a) == 0 || len(b) == 0 {
		return 0.0
	}
	bMap := make(map[string]CultureGene)
	for _, g := range b {
		bMap[g.Category+":"+g.Name] = g
	}

	matchSum := 0.0
	for _, ga := range a {
		key := ga.Category + ":" + ga.Name
		if gb, ok := bMap[key]; ok {
			diff := math.Abs(ga.Strength - gb.Strength)
			matchSum += 1.0 - diff
		}
	}

	total := math.Max(float64(len(a)), float64(len(b)))
	if total == 0 {
		return 1.0
	}
	return matchSum / total
}

func countUniqueGenes(genes []CultureGene, others [][]CultureGene) (unique, common int) {
	otherSet := make(map[string]bool)
	for _, og := range others {
		for _, g := range og {
			otherSet[g.Category+":"+g.Name] = true
		}
	}
	for _, g := range genes {
		key := g.Category + ":" + g.Name
		if otherSet[key] {
			common++
		} else {
			unique++
		}
	}
	return
}

func computeFingerprint(genes []CultureGene) string {
	if len(genes) == 0 {
		return "empty"
	}
	total := 0.0
	for _, g := range genes {
		total += g.Strength * float64(len(g.Name))
	}
	return fmt.Sprintf("fp-%d-g%d-%.4f", len(genes), int(total*1000)%99991, total)
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
