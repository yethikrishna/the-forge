// Package fuse provides multi-agent knowledge fusion.
// When multiple agents produce outputs about the same topic, fuse merges
// them into a single coherent result: deduplication, conflict resolution,
// confidence-weighted merging, and consensus building.
//
// Many minds, one answer.
package fuse

import (
	"crypto/sha256"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

// MergeStrategy defines how to merge contributions.
type MergeStrategy string

const (
	MergeVote      MergeStrategy = "vote"      // Majority wins
	MergeWeighted  MergeStrategy = "weighted"  // Weight by confidence
	MergeConcat    MergeStrategy = "concat"    // Concatenate all
	MergeBest      MergeStrategy = "best"      // Pick highest confidence
	MergeConsensus MergeStrategy = "consensus" // Require agreement
)

// Contribution represents a single agent's contribution.
type Contribution struct {
	ID         string            `json:"id"`
	AgentID    string            `json:"agent_id"`
	Topic      string            `json:"topic"`
	Content    string            `json:"content"`
	Confidence float64           `json:"confidence"` // 0-1
	Source     string            `json:"source,omitempty"`
	Tags       []string          `json:"tags,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	Timestamp  time.Time         `json:"timestamp"`
}

// FusedResult is the merged output.
type FusedResult struct {
	ID            string        `json:"id"`
	Topic         string        `json:"topic"`
	Content       string        `json:"content"`
	Strategy      MergeStrategy `json:"strategy"`
	Contributions int           `json:"contributions"`
	Confidence    float64       `json:"confidence"`
	Agreement     float64       `json:"agreement"` // 0-1, how much agents agreed
	ConflictCount int           `json:"conflict_count"`
	Contributors  []string      `json:"contributors"`
	Timestamp     time.Time     `json:"timestamp"`
}

// Conflict represents a disagreement between agents.
type Conflict struct {
	Topic      string   `json:"topic"`
	AgentIDs   []string `json:"agent_ids"`
	Contents   []string `json:"contents"`
	Resolution string   `json:"resolution"`
}

// Fuse handles knowledge fusion.
type Fuse struct {
	contributions map[string][]Contribution // topic -> contributions
	results       map[string]*FusedResult   // topic -> fused result
	mu            sync.RWMutex
}

// NewFuse creates a new knowledge fuser.
func NewFuse() *Fuse {
	return &Fuse{
		contributions: make(map[string][]Contribution),
		results:       make(map[string]*FusedResult),
	}
}

// Contribute adds a contribution from an agent.
func (f *Fuse) Contribute(agentID, topic, content string, confidence float64, tags []string) *Contribution {
	f.mu.Lock()
	defer f.mu.Unlock()

	c := Contribution{
		ID:         fmt.Sprintf("c-%x", sha256.Sum256([]byte(fmt.Sprintf("%s-%s-%d", agentID, topic, time.Now().UnixNano()))))[:16],
		AgentID:    agentID,
		Topic:      topic,
		Content:    content,
		Confidence: clampConfidence(confidence),
		Tags:       tags,
		Timestamp:  time.Now(),
	}

	f.contributions[topic] = append(f.contributions[topic], c)
	return &c
}

// Merge fuses contributions for a topic.
func (f *Fuse) Merge(topic string, strategy MergeStrategy) (*FusedResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	contribs, ok := f.contributions[topic]
	if !ok || len(contribs) == 0 {
		return nil, fmt.Errorf("no contributions for topic %q", topic)
	}

	result := &FusedResult{
		ID:            fmt.Sprintf("f-%x", sha256.Sum256([]byte(topic)))[:16],
		Topic:         topic,
		Strategy:      strategy,
		Contributions: len(contribs),
		Timestamp:     time.Now(),
	}

	// Collect contributors
	seen := make(map[string]bool)
	for _, c := range contribs {
		if !seen[c.AgentID] {
			result.Contributors = append(result.Contributors, c.AgentID)
			seen[c.AgentID] = true
		}
	}

	// Compute agreement
	result.Agreement = computeAgreement(contribs)

	// Detect conflicts
	conflicts := detectConflicts(contribs)
	result.ConflictCount = len(conflicts)

	// Apply merge strategy
	switch strategy {
	case MergeVote:
		result.Content, result.Confidence = mergeVote(contribs)
	case MergeWeighted:
		result.Content, result.Confidence = mergeWeighted(contribs)
	case MergeConcat:
		result.Content, result.Confidence = mergeConcat(contribs)
	case MergeBest:
		result.Content, result.Confidence = mergeBest(contribs)
	case MergeConsensus:
		content, conf, err := mergeConsensus(contribs)
		if err != nil {
			return nil, err
		}
		result.Content = content
		result.Confidence = conf
	default:
		result.Content, result.Confidence = mergeWeighted(contribs)
	}

	f.results[topic] = result
	return result, nil
}

// GetResult returns a fused result for a topic.
func (f *Fuse) GetResult(topic string) (*FusedResult, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	r, ok := f.results[topic]
	if !ok {
		return nil, false
	}
	copy := *r
	return &copy, true
}

// ListTopics returns all topics with contributions.
func (f *Fuse) ListTopics() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()

	topics := make([]string, 0, len(f.contributions))
	for t := range f.contributions {
		topics = append(topics, t)
	}
	sort.Strings(topics)
	return topics
}

// GetContributions returns contributions for a topic.
func (f *Fuse) GetContributions(topic string) []Contribution {
	f.mu.RLock()
	defer f.mu.RUnlock()

	contribs, ok := f.contributions[topic]
	if !ok {
		return nil
	}
	result := make([]Contribution, len(contribs))
	copy(result, contribs)
	return result
}

// Conflicts returns detected conflicts for a topic.
func (f *Fuse) Conflicts(topic string) []Conflict {
	f.mu.RLock()
	defer f.mu.RUnlock()

	contribs, ok := f.contributions[topic]
	if !ok {
		return nil
	}
	return detectConflicts(contribs)
}

// Stats returns fusion statistics.
func (f *Fuse) Stats() map[string]interface{} {
	f.mu.RLock()
	defer f.mu.RUnlock()

	totalContribs := 0
	for _, contribs := range f.contributions {
		totalContribs += len(contribs)
	}

	return map[string]interface{}{
		"topics":              len(f.contributions),
		"total_contributions": totalContribs,
		"fused_results":       len(f.results),
	}
}

// Merge strategies

func mergeVote(contribs []Contribution) (string, float64) {
	// Count occurrences of each content
	counts := make(map[string]int)
	confSum := make(map[string]float64)
	for _, c := range contribs {
		key := normalizeContent(c.Content)
		counts[key]++
		confSum[key] += c.Confidence
	}

	var bestKey string
	bestCount := 0
	for key, count := range counts {
		if count > bestCount {
			bestCount = count
			bestKey = key
		}
	}

	// Return original content (not normalized)
	for _, c := range contribs {
		if normalizeContent(c.Content) == bestKey {
			conf := confSum[bestKey] / float64(counts[bestKey])
			return c.Content, conf
		}
	}
	return "", 0
}

func mergeWeighted(contribs []Contribution) (string, float64) {
	if len(contribs) == 0 {
		return "", 0
	}

	var best *Contribution
	bestScore := -1.0
	for i := range contribs {
		score := contribs[i].Confidence
		if score > bestScore {
			bestScore = score
			best = &contribs[i]
		}
	}

	return best.Content, best.Confidence
}

func mergeConcat(contribs []Contribution) (string, float64) {
	var parts []string
	totalConf := 0.0
	for _, c := range contribs {
		parts = append(parts, fmt.Sprintf("[%s]: %s", c.AgentID, c.Content))
		totalConf += c.Confidence
	}

	avgConf := totalConf / float64(len(contribs))
	return strings.Join(parts, "\n---\n"), avgConf
}

func mergeBest(contribs []Contribution) (string, float64) {
	if len(contribs) == 0 {
		return "", 0
	}

	sort.Slice(contribs, func(i, j int) bool {
		return contribs[i].Confidence > contribs[j].Confidence
	})

	return contribs[0].Content, contribs[0].Confidence
}

func mergeConsensus(contribs []Contribution) (string, float64, error) {
	if len(contribs) < 2 {
		return "", 0, fmt.Errorf("consensus requires at least 2 contributions, got %d", len(contribs))
	}

	agreement := computeAgreement(contribs)
	if agreement < 0.8 {
		return "", 0, fmt.Errorf("no consensus: agreement %.1f%% (need 80%%)", agreement*100)
	}

	// Use weighted merge since there's agreement
	content, conf := mergeWeighted(contribs)
	return content, conf, nil
}

// Helpers

func computeAgreement(contribs []Contribution) float64 {
	if len(contribs) <= 1 {
		return 1.0
	}

	normalized := make(map[string]int)
	for _, c := range contribs {
		key := normalizeContent(c.Content)
		normalized[key]++
	}

	maxCount := 0
	for _, count := range normalized {
		if count > maxCount {
			maxCount = count
		}
	}

	return float64(maxCount) / float64(len(contribs))
}

func detectConflicts(contribs []Contribution) []Conflict {
	if len(contribs) <= 1 {
		return nil
	}

	// Group by normalized content
	groups := make(map[string][]Contribution)
	for _, c := range contribs {
		key := normalizeContent(c.Content)
		groups[key] = append(groups[key], c)
	}

	if len(groups) <= 1 {
		return nil // all agree
	}

	var conflicts []Conflict
	for _, group := range groups {
		agentIDs := make([]string, len(group))
		contents := make([]string, len(group))
		for i, c := range group {
			agentIDs[i] = c.AgentID
			contents[i] = c.Content
		}
		conflicts = append(conflicts, Conflict{
			AgentIDs: agentIDs,
			Contents: contents,
		})
	}
	return conflicts
}

func normalizeContent(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func clampConfidence(c float64) float64 {
	return math.Max(0, math.Min(1, c))
}
