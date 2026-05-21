// Package hnsw implements Hierarchical Navigable Small World graphs
// for approximate nearest neighbor search. Find any blade in the armory
// in logarithmic time.
package hnsw

import (
	"container/heap"
	"math"
	"math/rand"
	"sync"
)

// DistanceFunc computes the distance between two vectors.
type DistanceFunc func(a, b []float64) float64

// Euclidean computes Euclidean (L2) distance.
func Euclidean(a, b []float64) float64 {
	var sum float64
	for i := range a {
		d := a[i] - b[i]
		sum += d * d
	}
	return math.Sqrt(sum)
}

// Cosine computes cosine distance (1 - cosine similarity).
func Cosine(a, b []float64) float64 {
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 1.0
	}
	return 1.0 - dot/(math.Sqrt(normA)*math.Sqrt(normB))
}

// Node represents a vector in the HNSW graph.
type Node struct {
	ID     int
	Vector []float64
	Level  int
}

// Graph is an HNSW index.
type Graph struct {
	mu       sync.RWMutex
	nodes    map[int]*Node
	links    map[int]map[int][]int // level -> nodeID -> neighborIDs
	maxLevel int
	m        int // Max connections per layer
	mMax0    int // Max connections at layer 0
	efSearch int // Search width
	dist     DistanceFunc
	dims     int
	rng      *rand.Rand
}

// Config holds HNSW configuration.
type Config struct {
	M        int          // Max connections per layer (default 16)
	MMax0    int          // Max connections at layer 0 (default 2*M)
	EfSearch int          // Search width (default 200)
	Distance DistanceFunc // Distance function (default Euclidean)
	Dims     int          // Vector dimensions
}

// DefaultConfig returns sensible defaults.
func DefaultConfig(dims int) Config {
	m := 16
	return Config{
		M:        m,
		MMax0:    2 * m,
		EfSearch: 200,
		Distance: Euclidean,
		Dims:     dims,
	}
}

// New creates a new HNSW graph.
func New(config Config) *Graph {
	if config.M <= 0 {
		config.M = 16
	}
	if config.MMax0 <= 0 {
		config.MMax0 = 2 * config.M
	}
	if config.EfSearch <= 0 {
		config.EfSearch = 200
	}
	if config.Distance == nil {
		config.Distance = Euclidean
	}

	return &Graph{
		nodes:    make(map[int]*Node),
		links:    make(map[int]map[int][]int),
		maxLevel: -1,
		m:        config.M,
		mMax0:    config.MMax0,
		efSearch: config.EfSearch,
		dist:     config.Distance,
		dims:     config.Dims,
		rng:      rand.New(rand.NewSource(42)),
	}
}

// Insert adds a vector to the graph.
func (g *Graph) Insert(id int, vector []float64) {
	g.mu.Lock()
	defer g.mu.Unlock()

	level := g.randomLevel()
	node := &Node{ID: id, Vector: vector, Level: level}
	g.nodes[id] = node

	// Initialize links for all levels
	for l := 0; l <= level; l++ {
		if g.links[l] == nil {
			g.links[l] = make(map[int][]int)
		}
		g.links[l][id] = []int{}
	}

	if len(g.nodes) == 1 {
		g.maxLevel = level
		return
	}

	// Find entry point
	entryID := g.findEntryPoint()

	// Traverse from top to the node's level
	curID := entryID
	for l := g.maxLevel; l > level; l-- {
		curID = g.greedySearch(curID, vector, l)
	}

	// Insert at each level from the node's level down to 0
	for l := min(level, g.maxLevel); l >= 0; l-- {
		neighbors := g.searchLayer(curID, vector, g.m, l)
		mMax := g.mMax0
		if l > 0 {
			mMax = g.m
		}

		// Select neighbors
		selected := g.selectNeighbors(neighbors, mMax)

		// Add bidirectional links
		g.links[l][id] = []int{}
		for _, n := range selected {
			g.links[l][id] = append(g.links[l][id], n.ID)
			g.links[l][n.ID] = append(g.links[l][n.ID], id)

			// Prune if too many connections
			if len(g.links[l][n.ID]) > mMax {
				g.links[l][n.ID] = g.pruneConnections(n.ID, g.links[l][n.ID], l, mMax)
			}
		}

		if len(selected) > 0 {
			curID = selected[0].ID
		}
	}

	if level > g.maxLevel {
		g.maxLevel = level
	}
}

// Search finds the k nearest neighbors.
func (g *Graph) Search(vector []float64, k int) []Result {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if len(g.nodes) == 0 {
		return nil
	}

	entryID := g.findEntryPoint()

	// Traverse from top to level 1
	curID := entryID
	for l := g.maxLevel; l > 0; l-- {
		curID = g.greedySearch(curID, vector, l)
	}

	// Search at level 0 with ef width
	candidates := g.searchLayer(curID, vector, max(k, g.efSearch), 0)

	// Return top-k
	results := make([]Result, 0, k)
	for i := 0; i < min(k, len(candidates)); i++ {
		results = append(results, candidates[i])
	}

	return results
}

// Result represents a search result.
type Result struct {
	ID       int
	Distance float64
}

// candidateHeap is a max-heap of candidates by distance.
type candidateHeap []Result

func (h candidateHeap) Len() int           { return len(h) }
func (h candidateHeap) Less(i, j int) bool { return h[i].Distance > h[j].Distance } // max-heap
func (h candidateHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *candidateHeap) Push(x any)        { *h = append(*h, x.(Result)) }
func (h *candidateHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

func (g *Graph) randomLevel() int {
	level := 0
	ml := 1.0 / math.Log(float64(g.m))
	for g.rng.Float64() < math.Exp(-ml) && level < 32 {
		level++
	}
	return level
}

func (g *Graph) findEntryPoint() int {
	for id, node := range g.nodes {
		if node.Level >= g.maxLevel {
			return id
		}
	}
	// Fallback: return any node
	for id := range g.nodes {
		return id
	}
	return 0
}

func (g *Graph) greedySearch(entryID int, query []float64, level int) int {
	curID := entryID
	curDist := g.dist(g.nodes[curID].Vector, query)

	for {
		improved := false
		neighbors := g.links[level][curID]
		for _, nID := range neighbors {
			if n, ok := g.nodes[nID]; ok {
				d := g.dist(n.Vector, query)
				if d < curDist {
					curDist = d
					curID = nID
					improved = true
				}
			}
		}
		if !improved {
			break
		}
	}

	return curID
}

func (g *Graph) searchLayer(entryID int, query []float64, ef int, level int) []Result {
	visited := map[int]bool{entryID: true}
	entryDist := g.dist(g.nodes[entryID].Vector, query)

	candidates := &candidateHeap{{ID: entryID, Distance: entryDist}}
	heap.Init(candidates)

	results := &candidateHeap{{ID: entryID, Distance: entryDist}}
	heap.Init(results)

	for candidates.Len() > 0 {
		c := heap.Pop(candidates).(Result)
		furthest := (*results)[0]

		if c.Distance > furthest.Distance {
			break
		}

		neighbors := g.links[level][c.ID]
		for _, nID := range neighbors {
			if visited[nID] {
				continue
			}
			visited[nID] = true

			if n, ok := g.nodes[nID]; ok {
				d := g.dist(n.Vector, query)
				furthest = (*results)[0]

				if d < furthest.Distance || results.Len() < ef {
					heap.Push(candidates, Result{ID: nID, Distance: d})
					heap.Push(results, Result{ID: nID, Distance: d})

					if results.Len() > ef {
						heap.Pop(results)
					}
				}
			}
		}
	}

	// Convert heap to sorted slice
	sorted := make([]Result, results.Len())
	for i := results.Len() - 1; i >= 0; i-- {
		sorted[i] = heap.Pop(results).(Result)
	}
	// Sort by distance ascending
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].Distance < sorted[i].Distance {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	return sorted
}

func (g *Graph) selectNeighbors(candidates []Result, m int) []Result {
	if len(candidates) <= m {
		return candidates
	}
	return candidates[:m]
}

func (g *Graph) pruneConnections(nodeID int, connections []int, level, mMax int) []int {
	if len(connections) <= mMax {
		return connections
	}

	// Simple pruning: keep closest mMax neighbors
	node := g.nodes[nodeID]
	type connDist struct {
		id   int
		dist float64
	}

	dists := make([]connDist, len(connections))
	for i, cID := range connections {
		dists[i] = connDist{id: cID, dist: g.dist(node.Vector, g.nodes[cID].Vector)}
	}

	// Sort by distance
	for i := 0; i < len(dists); i++ {
		for j := i + 1; j < len(dists); j++ {
			if dists[j].dist < dists[i].dist {
				dists[i], dists[j] = dists[j], dists[i]
			}
		}
	}

	result := make([]int, mMax)
	for i := 0; i < mMax; i++ {
		result[i] = dists[i].id
	}
	return result
}

// Len returns the number of vectors in the graph.
func (g *Graph) Len() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.nodes)
}

// MaxLevel returns the highest level in the graph.
func (g *Graph) MaxLevel() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.maxLevel
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
