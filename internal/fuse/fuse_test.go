package fuse_test

import (
	"testing"

	"github.com/forge/sword/internal/fuse"
)

func TestContribute(t *testing.T) {
	f := fuse.NewFuse()
	c := f.Contribute("agent-1", "capital-france", "Paris", 0.95, nil)

	if c.AgentID != "agent-1" {
		t.Errorf("expected agent-1, got %s", c.AgentID)
	}
	if c.Confidence != 0.95 {
		t.Errorf("expected 0.95, got %f", c.Confidence)
	}
}

func TestMergeVote(t *testing.T) {
	f := fuse.NewFuse()
	f.Contribute("agent-1", "capital-france", "Paris", 0.9, nil)
	f.Contribute("agent-2", "capital-france", "Paris", 0.85, nil)
	f.Contribute("agent-3", "capital-france", "Lyon", 0.6, nil)

	result, err := f.Merge("capital-france", fuse.MergeVote)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Content != "Paris" {
		t.Errorf("expected Paris, got %s", result.Content)
	}
	if result.Contributions != 3 {
		t.Errorf("expected 3 contributions, got %d", result.Contributions)
	}
}

func TestMergeBest(t *testing.T) {
	f := fuse.NewFuse()
	f.Contribute("agent-1", "answer", "maybe", 0.5, nil)
	f.Contribute("agent-2", "answer", "definitely", 0.99, nil)

	result, err := f.Merge("answer", fuse.MergeBest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content != "definitely" {
		t.Errorf("expected 'definitely', got %s", result.Content)
	}
}

func TestMergeWeighted(t *testing.T) {
	f := fuse.NewFuse()
	f.Contribute("agent-1", "test", "low confidence", 0.3, nil)
	f.Contribute("agent-2", "test", "high confidence", 0.9, nil)

	result, err := f.Merge("test", fuse.MergeWeighted)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content != "high confidence" {
		t.Errorf("expected high confidence answer, got %s", result.Content)
	}
}

func TestMergeConcat(t *testing.T) {
	f := fuse.NewFuse()
	f.Contribute("a1", "topic", "first", 0.8, nil)
	f.Contribute("a2", "topic", "second", 0.7, nil)

	result, err := f.Merge("topic", fuse.MergeConcat)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Contributions != 2 {
		t.Errorf("expected 2 contributions, got %d", result.Contributions)
	}
}

func TestMergeConsensus(t *testing.T) {
	f := fuse.NewFuse()
	f.Contribute("a1", "topic", "agree", 0.9, nil)
	f.Contribute("a2", "topic", "agree", 0.85, nil)

	result, err := f.Merge("topic", fuse.MergeConsensus)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content != "agree" {
		t.Errorf("expected 'agree', got %s", result.Content)
	}
}

func TestMergeConsensusFail(t *testing.T) {
	f := fuse.NewFuse()
	f.Contribute("a1", "topic", "yes", 0.9, nil)
	f.Contribute("a2", "topic", "no", 0.9, nil)

	_, err := f.Merge("topic", fuse.MergeConsensus)
	if err == nil {
		t.Error("expected consensus to fail with disagreeing agents")
	}
}

func TestMergeNoContributions(t *testing.T) {
	f := fuse.NewFuse()
	_, err := f.Merge("nonexistent", fuse.MergeVote)
	if err == nil {
		t.Error("expected error for nonexistent topic")
	}
}

func TestListTopics(t *testing.T) {
	f := fuse.NewFuse()
	f.Contribute("a1", "topic-a", "x", 0.9, nil)
	f.Contribute("a2", "topic-b", "y", 0.8, nil)

	topics := f.ListTopics()
	if len(topics) != 2 {
		t.Errorf("expected 2 topics, got %d", len(topics))
	}
}

func TestGetContributions(t *testing.T) {
	f := fuse.NewFuse()
	f.Contribute("a1", "topic", "x", 0.9, nil)
	f.Contribute("a2", "topic", "y", 0.8, nil)

	contribs := f.GetContributions("topic")
	if len(contribs) != 2 {
		t.Errorf("expected 2 contributions, got %d", len(contribs))
	}
}

func TestConflicts(t *testing.T) {
	f := fuse.NewFuse()
	f.Contribute("a1", "topic", "yes", 0.9, nil)
	f.Contribute("a2", "topic", "no", 0.9, nil)

	conflicts := f.Conflicts("topic")
	if len(conflicts) == 0 {
		t.Error("expected conflicts")
	}
}

func TestNoConflicts(t *testing.T) {
	f := fuse.NewFuse()
	f.Contribute("a1", "topic", "same", 0.9, nil)
	f.Contribute("a2", "topic", "same", 0.9, nil)

	conflicts := f.Conflicts("topic")
	if len(conflicts) != 0 {
		t.Error("expected no conflicts when agents agree")
	}
}

func TestStats(t *testing.T) {
	f := fuse.NewFuse()
	f.Contribute("a1", "topic-a", "x", 0.9, nil)
	f.Contribute("a2", "topic-b", "y", 0.8, nil)

	stats := f.Stats()
	if stats["topics"].(int) != 2 {
		t.Errorf("expected 2 topics, got %v", stats["topics"])
	}
	if stats["total_contributions"].(int) != 2 {
		t.Errorf("expected 2 contributions, got %v", stats["total_contributions"])
	}
}

func TestConfidenceClamping(t *testing.T) {
	f := fuse.NewFuse()
	c := f.Contribute("a1", "topic", "x", 1.5, nil)
	if c.Confidence != 1.0 {
		t.Errorf("expected 1.0, got %f", c.Confidence)
	}

	c2 := f.Contribute("a2", "topic", "y", -0.5, nil)
	if c2.Confidence != 0.0 {
		t.Errorf("expected 0.0, got %f", c2.Confidence)
	}
}
