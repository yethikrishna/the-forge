package hnsw_test

import (
	"math"
	"math/rand"
	"testing"

	"github.com/forge/sword/internal/hnsw"
)

func TestNewGraph(t *testing.T) {
	g := hnsw.New(hnsw.DefaultConfig(3))
	if g == nil {
		t.Fatal("graph should not be nil")
	}
	if g.Len() != 0 {
		t.Errorf("expected 0 nodes, got %d", g.Len())
	}
}

func TestInsertAndSearch(t *testing.T) {
	g := hnsw.New(hnsw.DefaultConfig(2))

	// Insert some 2D vectors
	vectors := map[int][]float64{
		0: {0.0, 0.0},
		1: {1.0, 0.0},
		2: {0.0, 1.0},
		3: {1.0, 1.0},
		4: {0.5, 0.5},
	}

	for id, vec := range vectors {
		g.Insert(id, vec)
	}

	if g.Len() != 5 {
		t.Errorf("expected 5 nodes, got %d", g.Len())
	}

	// Search for nearest neighbor to [0.1, 0.1]
	results := g.Search([]float64{0.1, 0.1}, 1)
	if len(results) == 0 {
		t.Fatal("expected at least 1 result")
	}

	// Nearest should be [0,0] (id=0)
	if results[0].ID != 0 {
		t.Errorf("expected nearest to be id=0, got id=%d (dist=%.3f)", results[0].ID, results[0].Distance)
	}
}

func TestSearchKNN(t *testing.T) {
	g := hnsw.New(hnsw.DefaultConfig(2))

	// Insert vectors along a line
	for i := 0; i < 100; i++ {
		g.Insert(i, []float64{float64(i), 0.0})
	}

	// Search for 5 nearest neighbors to x=50
	results := g.Search([]float64{50.0, 0.0}, 5)
	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}

	// First result should be very close to 50
	if results[0].Distance > 2.0 {
		t.Errorf("nearest neighbor distance too large: %.3f", results[0].Distance)
	}
}

func TestCosineDistance(t *testing.T) {
	a := []float64{1.0, 0.0}
	b := []float64{0.0, 1.0}
	c := []float64{1.0, 0.0}

	distAB := hnsw.Cosine(a, b)
	distAC := hnsw.Cosine(a, c)

	// a and b are orthogonal: distance should be 1.0
	if math.Abs(distAB-1.0) > 0.01 {
		t.Errorf("cosine distance of orthogonal vectors should be ~1.0, got %.3f", distAB)
	}

	// a and c are identical: distance should be 0.0
	if math.Abs(distAC) > 0.01 {
		t.Errorf("cosine distance of identical vectors should be ~0.0, got %.3f", distAC)
	}
}

func TestEuclideanDistance(t *testing.T) {
	a := []float64{0.0, 0.0}
	b := []float64{3.0, 4.0}

	dist := hnsw.Euclidean(a, b)
	if math.Abs(dist-5.0) > 0.01 {
		t.Errorf("euclidean distance should be 5.0, got %.3f", dist)
	}
}

func TestEmptyGraphSearch(t *testing.T) {
	g := hnsw.New(hnsw.DefaultConfig(2))
	results := g.Search([]float64{0.0, 0.0}, 5)
	if len(results) != 0 {
		t.Errorf("empty graph should return no results, got %d", len(results))
	}
}

func BenchmarkInsert(b *testing.B) {
	g := hnsw.New(hnsw.DefaultConfig(128))
	rng := rand.New(rand.NewSource(42))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		vec := make([]float64, 128)
		for j := range vec {
			vec[j] = rng.NormFloat64()
		}
		g.Insert(i, vec)
	}
}

func BenchmarkSearch(b *testing.B) {
	g := hnsw.New(hnsw.DefaultConfig(128))
	rng := rand.New(rand.NewSource(42))

	// Pre-populate
	for i := 0; i < 1000; i++ {
		vec := make([]float64, 128)
		for j := range vec {
			vec[j] = rng.NormFloat64()
		}
		g.Insert(i, vec)
	}

	query := make([]float64, 128)
	for j := range query {
		query[j] = rng.NormFloat64()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.Search(query, 10)
	}
}
