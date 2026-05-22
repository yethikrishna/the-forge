package science

import (
	"os"
	"path/filepath"
	"testing"
)

func tempFile(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "science.json")
}

func TestStoreLoadSave(t *testing.T) {
	s := NewStore(tempFile(t))
	s.Hypotheses = append(s.Hypotheses, Hypothesis{ID: "hy_1", Statement: "X causes Y"})
	if err := s.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	s2 := NewStore(s.filePath)
	if err := s2.Load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(s2.Hypotheses) != 1 || s2.Hypotheses[0].Statement != "X causes Y" {
		t.Errorf("unexpected after load")
	}
}

func TestFormHypothesis(t *testing.T) {
	hy := FormHypothesis("Microservices improve deploy frequency", "architecture",
		[]string{"deploy freq increases >50%", "incident rate stays flat"})
	if hy.Status != "proposed" {
		t.Errorf("expected proposed, got %s", hy.Status)
	}
	if hy.Confidence != 0.5 {
		t.Errorf("expected initial confidence 0.5, got %.2f", hy.Confidence)
	}
	if len(hy.Predictions) != 2 {
		t.Errorf("expected 2 predictions, got %d", len(hy.Predictions))
	}
}

func TestRunExperiment(t *testing.T) {
	ex := RunExperiment("hy_1", "A/B test deploy freq", "split 50/50 for 4 weeks",
		map[string]string{"team_size": "control", "architecture": "experiment"})
	if ex.Status != "running" {
		t.Errorf("expected running, got %s", ex.Status)
	}
	if ex.HypothesisID != "hy_1" {
		t.Errorf("expected hy_1, got %s", ex.HypothesisID)
	}
}

func TestPeerReview(t *testing.T) {
	rv := PeerReview("hy_1", "Dr. Smith", "accept", "Solid methodology",
		0.9, 0.85, 0.7)
	if rv.Verdict != "accept" {
		t.Errorf("expected accept, got %s", rv.Verdict)
	}
	if rv.RigorScore != 0.9 {
		t.Errorf("expected 0.9 rigor, got %.2f", rv.RigorScore)
	}
}

func TestPeerReview_Reject(t *testing.T) {
	rv := PeerReview("hy_2", "Dr. Jones", "reject", "Insufficient sample size",
		0.3, 0.5, 0.4)
	if rv.Verdict != "reject" {
		t.Errorf("expected reject, got %s", rv.Verdict)
	}
}

func TestCheckReproducibility(t *testing.T) {
	rs := CheckReproducibility("finding_1", 5, 4, "One failure due to env diff")
	if rs.Score != 0.8 {
		t.Errorf("expected 0.8, got %.2f", rs.Score)
	}
	if rs.AttemptCount != 5 || rs.SuccessCount != 4 {
		t.Errorf("unexpected counts: %d/%d", rs.SuccessCount, rs.AttemptCount)
	}
}

func TestCheckReproducibility_ZeroAttempts(t *testing.T) {
	rs := CheckReproducibility("finding_2", 0, 0, "Not yet attempted")
	if rs.Score != 0 {
		t.Errorf("expected 0 for zero attempts, got %.2f", rs.Score)
	}
}

func TestCheckReproducibility_Perfect(t *testing.T) {
	rs := CheckReproducibility("finding_3", 3, 3, "All reproductions successful")
	if rs.Score != 1.0 {
		t.Errorf("expected 1.0, got %.2f", rs.Score)
	}
}

func TestGenerateScienceReport(t *testing.T) {
	s := NewStore(tempFile(t))
	s.Experiments = append(s.Experiments, Experiment{ID: "ex_1"})
	s.ResearchProjects = append(s.ResearchProjects, ResearchProject{ID: "rp_1"})
	report := GenerateScienceReport(s)
	if len(report.Experiments) != 1 || len(report.ResearchProjects) != 1 {
		t.Errorf("unexpected report contents")
	}
	if report.GeneratedAt.IsZero() {
		t.Error("expected non-zero generated_at")
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
