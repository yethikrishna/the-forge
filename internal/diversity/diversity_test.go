package diversity

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func init() {
	// Seed RNG for deterministic-ish tests
	rand.Seed(42)
}

func rngFloat() float64 { return rand.Float64() }

func generateID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UTC().UnixNano())
}

func tempStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s := NewStore(filepath.Join(dir, "diversity.json"))
	if err := s.Load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	return s
}

func makeGenes(prefix string, n int) []CultureGene {
	genes := make([]CultureGene, n)
	categories := []string{"communication", "decision", "ritual", "structure", "innovation"}
	for i := 0; i < n; i++ {
		genes[i] = CultureGene{
			ID:         fmt.Sprintf("%s-gene-%d", prefix, i),
			Name:       fmt.Sprintf("%s-trait-%d", prefix, i),
			Category:   categories[i%len(categories)],
			Expression: fmt.Sprintf("expresses as %s behavior %d", prefix, i),
			Strength:   float64(i+1) / float64(n+1),
		}
	}
	return genes
}

func TestGenerateSeed_NoSource(t *testing.T) {
	s := tempStore(t)
	seed := s.GenerateSeed("", 0.1, nil)
	if seed.ID == "" {
		t.Fatal("expected non-empty seed ID")
	}
	if len(seed.Genome) > 0 {
		t.Fatal("expected empty genome without source org")
	}
	if seed.MutationRate != 0.1 {
		t.Fatalf("expected mutation rate 0.1, got %f", seed.MutationRate)
	}
}

func TestGenerateSeed_WithSource(t *testing.T) {
	s := tempStore(t)
	dna := CultureDNA{
		OrgID: "org-a",
		Genes: makeGenes("a", 5),
	}
	s.PutDNA(dna)

	seed := s.GenerateSeed("org-a", 0.5, map[string]string{"region": "us-east"})
	if seed.SourceOrgID != "org-a" {
		t.Fatalf("expected source org-a, got %s", seed.SourceOrgID)
	}
	if len(seed.Genome) != 5 {
		t.Fatalf("expected 5 genes, got %d", len(seed.Genome))
	}
	if seed.Constraints["region"] != "us-east" {
		t.Fatal("expected constraint region=us-east")
	}
}

func TestMeasureDiversity_SingleOrg(t *testing.T) {
	s := tempStore(t)
	s.PutDNA(CultureDNA{OrgID: "solo", Genes: makeGenes("s", 4)})

	score, err := s.MeasureDiversity("solo")
	if err != nil {
		t.Fatal(err)
	}
	if score.Score != 1.0 {
		t.Fatalf("expected diversity 1.0 for sole org, got %f", score.Score)
	}
	if score.UniqueGenes != 4 {
		t.Fatalf("expected 4 unique genes, got %d", score.UniqueGenes)
	}
}

func TestMeasureDiversity_MultipleOrgs(t *testing.T) {
	s := tempStore(t)
	s.PutDNA(CultureDNA{OrgID: "org-a", Genes: makeGenes("a", 4)})
	s.PutDNA(CultureDNA{OrgID: "org-b", Genes: makeGenes("b", 4)})

	scoreA, _ := s.MeasureDiversity("org-a")
	scoreB, _ := s.MeasureDiversity("org-b")

	if scoreA.Score <= 0 || scoreA.Score > 1 {
		t.Fatalf("scoreA out of range: %f", scoreA.Score)
	}
	if scoreB.Score <= 0 || scoreB.Score > 1 {
		t.Fatalf("scoreB out of range: %f", scoreB.Score)
	}
}

func TestCompareCultures_Identical(t *testing.T) {
	s := tempStore(t)
	genes := makeGenes("x", 3)
	s.PutDNA(CultureDNA{OrgID: "a", Genes: genes})
	s.PutDNA(CultureDNA{OrgID: "b", Genes: append([]CultureGene{}, genes...)})

	sim := s.CompareCultures("a", "b")
	if sim < 0.99 {
		t.Fatalf("expected near-1.0 similarity for identical cultures, got %f", sim)
	}
}

func TestCompareCultures_Different(t *testing.T) {
	s := tempStore(t)
	s.PutDNA(CultureDNA{OrgID: "a", Genes: makeGenes("a", 3)})
	s.PutDNA(CultureDNA{OrgID: "b", Genes: makeGenes("b", 3)})

	sim := s.CompareCultures("a", "b")
	if sim >= 1.0 {
		t.Fatalf("expected <1.0 similarity for different cultures, got %f", sim)
	}
}

func TestCompareCultures_Missing(t *testing.T) {
	s := tempStore(t)
	sim := s.CompareCultures("ghost-a", "ghost-b")
	if sim != 0 {
		t.Fatalf("expected 0 similarity for missing orgs, got %f", sim)
	}
}

func TestEvolveCulture_NewOrg(t *testing.T) {
	s := tempStore(t)
	dna, err := s.EvolveCulture("new-org", "nonexistent-seed")
	if err != nil {
		t.Fatal(err)
	}
	if dna.OrgID != "new-org" {
		t.Fatal("expected org ID new-org")
	}
}

func TestEvolveCulture_WithSeed(t *testing.T) {
	s := tempStore(t)
	s.PutDNA(CultureDNA{OrgID: "org-x", Genes: makeGenes("x", 2)})
	seed := s.GenerateSeed("org-x", 0.0, nil)

	dna, err := s.EvolveCulture("target-org", seed.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(dna.Genes) != 2 {
		t.Fatalf("expected 2 genes from seed, got %d", len(dna.Genes))
	}
}

func TestGenerateDiversityReport(t *testing.T) {
	s := tempStore(t)
	s.PutDNA(CultureDNA{OrgID: "o1", Genes: makeGenes("o1", 3)})
	s.PutDNA(CultureDNA{OrgID: "o2", Genes: makeGenes("o2", 3)})
	s.MeasureDiversity("o1")
	s.MeasureDiversity("o2")

	report := s.GenerateDiversityReport()
	if report["org_count"] != 2 {
		t.Fatalf("expected 2 orgs, got %v", report["org_count"])
	}
	if report["seed_count"] != 0 {
		t.Fatalf("expected 0 seeds, got %v", report["seed_count"])
	}
}

func TestStorePersistence(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "diversity.json")

	s1 := NewStore(fp)
	s1.PutDNA(CultureDNA{OrgID: "persist", Genes: makeGenes("p", 2)})
	if err := s1.Save(); err != nil {
		t.Fatal(err)
	}

	s2 := NewStore(fp)
	if err := s2.Load(); err != nil {
		t.Fatal(err)
	}
	dna, ok := s2.GetDNA("persist")
	if !ok {
		t.Fatal("expected to find persisted DNA")
	}
	if len(dna.Genes) != 2 {
		t.Fatalf("expected 2 genes, got %d", len(dna.Genes))
	}
}

func TestStoreLoad_NoFile(t *testing.T) {
	s := NewStore("/tmp/nonexistent_diversity_test.json")
	err := s.Load()
	if err != nil {
		t.Fatalf("expected no error for missing file, got %v", err)
	}
	os.Remove("/tmp/nonexistent_diversity_test.json")
}
