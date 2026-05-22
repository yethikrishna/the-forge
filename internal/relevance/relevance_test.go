package relevance

import (
	"path/filepath"
	"testing"
)

func tempStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s := NewStore(filepath.Join(dir, "relevance.json"))
	if err := s.Load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	return s
}

func TestScoreRelevance_NoData(t *testing.T) {
	s := tempStore(t)
	rs := s.ScoreRelevance("p1")
	if rs.Score != 0 {
		t.Fatalf("expected 0 score with no data, got %f", rs.Score)
	}
}

func TestScoreRelevance_ActiveHuman(t *testing.T) {
	s := tempStore(t)
	s.DefineHumanZone(HumanZone{PersonID: "p1", ZoneName: "strategic", Priority: "critical"})
	s.TrackDecisions(DecisionRecord{
		PersonID:    "p1",
		Decision:    "Approve acquisition",
		Category:    "strategic",
		HumanChoice: true,
		Weight:      0.9,
	})
	s.TrackDecisions(DecisionRecord{
		PersonID:    "p1",
		Decision:    "Set Q3 priorities",
		Category:    "strategic",
		HumanChoice: true,
		Weight:      0.7,
	})

	rs := s.ScoreRelevance("p1")
	if rs.Score < 0.5 {
		t.Fatalf("expected high relevance for active human, got %f", rs.Score)
	}
	if rs.DecisionsRubberStampped != 0 {
		t.Fatal("expected 0 rubber stamps")
	}
}

func TestScoreRelevance_RubberStamp(t *testing.T) {
	s := tempStore(t)
	s.TrackDecisions(DecisionRecord{
		PersonID:    "p1",
		Decision:    "Approve auto-suggested budget",
		Category:    "financial",
		HumanChoice: false,
		Weight:      0.5,
	})

	rs := s.ScoreRelevance("p1")
	if rs.DecisionsRubberStampped != 1 {
		t.Fatal("expected 1 rubber stamp")
	}
}

func TestTrackDecisions(t *testing.T) {
	s := tempStore(t)
	d := s.TrackDecisions(DecisionRecord{
		PersonID:    "p1",
		Decision:    "Hire CTO",
		Category:    "strategic",
		HumanChoice: true,
		Weight:      0.8,
	})
	if d.ID == "" {
		t.Fatal("expected auto-generated ID")
	}
}

func TestDefineHumanZone(t *testing.T) {
	s := tempStore(t)
	z := s.DefineHumanZone(HumanZone{
		PersonID:    "p1",
		ZoneName:    "ethical",
		Description: "AI ethics decisions must involve human",
		Priority:    "critical",
	})
	if z.ID == "" {
		t.Fatal("expected auto-generated ID")
	}
}

func TestRouteMeaningfulTask(t *testing.T) {
	s := tempStore(t)
	task := s.RouteMeaningfulTask(MeaningfulTask{
		PersonID: "p1",
		ZoneName: "ethical",
		Task:     "Review AI bias report",
		Reason:   "Requires human ethical judgment",
		Priority: "high",
	})
	if task.ID == "" {
		t.Fatal("expected auto-generated ID")
	}
	if task.Status != "pending" {
		t.Fatalf("expected pending status, got %s", task.Status)
	}
}

func TestGenerateRelevanceReport(t *testing.T) {
	s := tempStore(t)
	s.TrackDecisions(DecisionRecord{PersonID: "p1", Decision: "D1", Weight: 0.5, HumanChoice: true})
	s.RouteMeaningfulTask(MeaningfulTask{PersonID: "p1", Task: "T1", Priority: "medium"})

	report := s.GenerateRelevanceReport()
	if report["total_decisions"] != 1 {
		t.Fatalf("expected 1 decision, got %v", report["total_decisions"])
	}
	if report["pending_tasks"] != 1 {
		t.Fatalf("expected 1 pending task, got %v", report["pending_tasks"])
	}
}

func TestStorePersistence(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "rel.json")

	s1 := NewStore(fp)
	s1.DefineHumanZone(HumanZone{PersonID: "p1", ZoneName: "strategic", Priority: "high"})
	if err := s1.Save(); err != nil {
		t.Fatal(err)
	}

	s2 := NewStore(fp)
	if err := s2.Load(); err != nil {
		t.Fatal(err)
	}
	report := s2.GenerateRelevanceReport()
	if report["human_zones"] != 1 {
		t.Fatalf("expected 1 zone after load, got %v", report["human_zones"])
	}
}
