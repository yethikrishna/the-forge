package playbook

import (
	"strings"
	"testing"
	"time"
)

func sampleLog() SessionLog {
	return SessionLog{
		SessionID: "sess-1",
		AgentID:   "coder",
		Goal:      "Fix authentication bug in login handler",
		Outcome:   "Fixed null pointer in auth middleware",
		Success:   true,
		Actions: []Action{
			{Type: "read", Target: "auth/middleware.go", Success: true, Duration: 100, Time: time.Now()},
			{Type: "search", Target: "null pointer auth", Success: true, Duration: 200, Time: time.Now()},
			{Type: "write", Target: "auth/middleware.go", Success: true, Duration: 300, Time: time.Now()},
			{Type: "execute", Target: "go test ./auth/", Success: true, Duration: 1500, Time: time.Now()},
		},
		Timestamp: time.Now(),
	}
}

func TestNewGenerator(t *testing.T) {
	g := NewGenerator("")
	if g == nil {
		t.Fatal("expected generator")
	}
}

func TestGenerate(t *testing.T) {
	g := NewGenerator("")
	pb, err := g.Generate(sampleLog())
	if err != nil {
		t.Fatal(err)
	}
	if pb.ID == "" {
		t.Error("expected ID")
	}
	if len(pb.Steps) != 4 {
		t.Errorf("expected 4 steps, got %d", len(pb.Steps))
	}
	if pb.Source != "auto" {
		t.Error("should be auto-generated")
	}
}

func TestGenerateEmptyActions(t *testing.T) {
	g := NewGenerator("")
	_, err := g.Generate(SessionLog{SessionID: "s1"})
	if err == nil {
		t.Error("should error with no actions")
	}
}

func TestGenerateSuccessRate(t *testing.T) {
	g := NewGenerator("")
	log := sampleLog()
	log.Actions[2].Success = false
	pb, _ := g.Generate(log)
	if pb.SuccessRate != 0.75 {
		t.Errorf("expected 0.75, got %.2f", pb.SuccessRate)
	}
}

func TestGenerateTags(t *testing.T) {
	g := NewGenerator("")
	pb, _ := g.Generate(sampleLog())
	found := false
	for _, tag := range pb.Tags {
		if tag == "successful" {
			found = true
		}
	}
	if !found {
		t.Error("should tag as successful")
	}
}

func TestCreate(t *testing.T) {
	g := NewGenerator("")
	pb := g.Create("Manual PB", "Test playbook", []Step{
		{Title: "Step 1", Action: "read", Success: true},
		{Title: "Step 2", Action: "write", Success: true},
	})

	if pb.Source != "manual" {
		t.Error("should be manual")
	}
	if pb.Steps[0].Index != 1 {
		t.Error("first step should be index 1")
	}
}

func TestGet(t *testing.T) {
	g := NewGenerator("")
	pb, _ := g.Generate(sampleLog())
	got, ok := g.Get(pb.ID)
	if !ok {
		t.Fatal("should find")
	}
	if got.Name != pb.Name {
		t.Error("name mismatch")
	}
}

func TestGetNotFound(t *testing.T) {
	g := NewGenerator("")
	_, ok := g.Get("nonexistent")
	if ok {
		t.Error("should not find")
	}
}

func TestList(t *testing.T) {
	g := NewGenerator("")
	g.Generate(sampleLog())
	log2 := sampleLog()
	log2.Goal = "Second playbook"
	g.Generate(log2)

	list := g.List()
	if len(list) != 2 {
		t.Errorf("expected 2, got %d", len(list))
	}
}

func TestSearch(t *testing.T) {
	g := NewGenerator("")
	g.Generate(sampleLog())

	results := g.Search("authentication")
	if len(results) == 0 {
		t.Error("should find by name")
	}
}

func TestSearchByTag(t *testing.T) {
	g := NewGenerator("")
	g.Generate(sampleLog())

	results := g.Search("successful")
	if len(results) == 0 {
		t.Error("should find by tag")
	}
}

func TestRecordUse(t *testing.T) {
	g := NewGenerator("")
	pb, _ := g.Generate(sampleLog())
	g.RecordUse(pb.ID)
	g.RecordUse(pb.ID)

	got, _ := g.Get(pb.ID)
	if got.Uses != 2 {
		t.Errorf("expected 2 uses, got %d", got.Uses)
	}
}

func TestRecordUseNotFound(t *testing.T) {
	g := NewGenerator("")
	err := g.RecordUse("nonexistent")
	if err == nil {
		t.Error("should error")
	}
}

func TestDelete(t *testing.T) {
	g := NewGenerator("")
	pb, _ := g.Generate(sampleLog())
	g.Delete(pb.ID)

	_, ok := g.Get(pb.ID)
	if ok {
		t.Error("should be deleted")
	}
}

func TestDeleteNotFound(t *testing.T) {
	g := NewGenerator("")
	err := g.Delete("nonexistent")
	if err == nil {
		t.Error("should error")
	}
}

func TestTop(t *testing.T) {
	g := NewGenerator("")
	_, _ = g.Generate(sampleLog())
	log2 := sampleLog()
	log2.Goal = "Second"
	pb2, _ := g.Generate(log2)
	g.RecordUse(pb2.ID)
	g.RecordUse(pb2.ID)

	top := g.Top(1)
	if len(top) != 1 {
		t.Fatal("expected 1")
	}
	if top[0].ID != pb2.ID {
		t.Error("most used should be first")
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	g1 := NewGenerator(dir)
	g1.Generate(sampleLog())

	g2 := NewGenerator(dir)
	list := g2.List()
	if len(list) != 1 {
		t.Fatal("playbook should persist")
	}
}

func TestFormatPlaybook(t *testing.T) {
	pb := &Playbook{
		Name:        "Fix auth bug",
		ID:          "pb-1",
		Source:      "auto",
		SuccessRate: 1.0,
		Uses:        5,
		Tags:        []string{"read", "write", "successful"},
		Steps: []Step{
			{Index: 1, Title: "Read middleware", Success: true},
			{Index: 2, Title: "Write fix", Success: true},
		},
	}

	s := FormatPlaybook(pb)
	if !strings.Contains(s, "Fix auth bug") {
		t.Error("should show name")
	}
	if !strings.Contains(s, "100%") {
		t.Error("should show success rate")
	}
	if !strings.Contains(s, "Read middleware") {
		t.Error("should show steps")
	}
}

func TestExtractNameTruncation(t *testing.T) {
	long := strings.Repeat("a", 100)
	name := extractName(long)
	if len(name) > 60 {
		t.Error("should truncate long names")
	}
}

func TestCalcSuccessRateEmpty(t *testing.T) {
	rate := calcSuccessRate(nil)
	if rate != 0 {
		t.Error("empty actions should be 0")
	}
}
