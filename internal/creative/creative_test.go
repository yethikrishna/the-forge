package creative

import (
	"os"
	"path/filepath"
	"testing"
)

func tempFile(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "creative.json")
}

func TestStoreLoadSave(t *testing.T) {
	s := NewStore(tempFile(t))
	s.BrandIdentities = append(s.BrandIdentities, BrandIdentity{ID: "bi_1", Name: "Acme"})
	if err := s.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	s2 := NewStore(s.filePath)
	if err := s2.Load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(s2.BrandIdentities) != 1 || s2.BrandIdentities[0].Name != "Acme" {
		t.Errorf("unexpected after load: %+v", s2.BrandIdentities)
	}
}

func TestJudgeAesthetics(t *testing.T) {
	dims := map[string]float64{
		"harmony":  0.8,
		"contrast": 0.7,
		"balance":  0.9,
		"novelty":  0.5,
		"clarity":  0.8,
	}
	as := JudgeAesthetics("homepage", "alice", dims)
	if as.Score < 0.5 || as.Score > 1.0 {
		t.Errorf("expected score between 0.5-1.0, got %.2f", as.Score)
	}
	if as.Subject != "homepage" {
		t.Errorf("expected homepage, got %s", as.Subject)
	}
	expected := (0.8 + 0.7 + 0.9 + 0.5 + 0.8) / 5.0
	if diff := as.Score - expected; diff > 0.001 || diff < -0.001 {
		t.Errorf("expected %.4f, got %.4f", expected, as.Score)
	}
}

func TestJudgeAesthetics_Empty(t *testing.T) {
	as := JudgeAesthetics("blank", "bob", map[string]float64{})
	if as.Score != 0 {
		t.Error("expected 0 score for empty dimensions, but got NaN or non-zero")
	}
}

func TestDevelopBrand(t *testing.T) {
	bi := DevelopBrand("Forge", "bold", "direct", "Build with fire",
		[]string{"craft", "speed"}, []string{"#FF0000", "#000000"})
	if bi.Name != "Forge" || bi.Personality != "bold" {
		t.Errorf("unexpected brand: %+v", bi)
	}
	if len(bi.Values) != 2 {
		t.Errorf("expected 2 values, got %d", len(bi.Values))
	}
}

func TestGenerateCreativeLeap(t *testing.T) {
	cl := GenerateCreativeLeap("architecture", "gothic", "software", "cathedrals and compilers share sacred geometry", 0.9, 0.6)
	if cl.Domain != "architecture" || cl.Surprise != 0.9 {
		t.Errorf("unexpected leap: %+v", cl)
	}
}

func TestTellStory(t *testing.T) {
	st := TellStory("The Pivot", "startup", "market crashed", "pivoted to AI", "adapt or die", "founders")
	if st.Protagonist != "startup" || st.Moral != "adapt or die" {
		t.Errorf("unexpected story: %+v", st)
	}
}

func TestDesignExperience(t *testing.T) {
	tps := []string{"landing", "onboarding", "first_use"}
	emotions := map[string]string{
		"landing":     "curiosity",
		"onboarding":  "confidence",
		"first_use":   "delight",
	}
	ed := DesignExperience("signup flow", tps, emotions)
	if ed.FlowScore != 1.0 {
		t.Errorf("expected flow_score 1.0 with all mapped, got %.2f", ed.FlowScore)
	}
}

func TestDesignExperience_PartialMapping(t *testing.T) {
	tps := []string{"landing", "onboarding", "first_use"}
	emotions := map[string]string{
		"landing": "curiosity",
	}
	ed := DesignExperience("partial flow", tps, emotions)
	if ed.FlowScore < 0.3 || ed.FlowScore > 0.4 {
		t.Errorf("expected ~0.33 flow_score, got %.2f", ed.FlowScore)
	}
}

func TestGenerateCreativeReport(t *testing.T) {
	s := NewStore(tempFile(t))
	s.Stories = append(s.Stories, Story{ID: "st_1"})
	report := GenerateCreativeReport(s)
	if len(report.Stories) != 1 {
		t.Errorf("expected 1 story in report")
	}
	if report.GeneratedAt.IsZero() {
		t.Error("expected non-zero generated_at")
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
