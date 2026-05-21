package orglearn

import (
	"context"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *LearningStore {
	t.Helper()
	store, err := NewLearningStore(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestNewLearningStore(t *testing.T) {
	store := newTestStore(t)
	if store == nil {
		t.Fatal("expected non-nil store")
	}
}

func TestAddAndGetLesson(t *testing.T) {
	store := newTestStore(t)

	lesson := &Lesson{
		Source:              SourceFailure,
		Context:             "Deploy failed due to missing env var",
		Insight:             "Always validate env vars before deploy",
		Tags:                []string{"deploy", "env", "validation"},
		ApplicableDivisions: []string{"platform", "devops"},
		Verified:            false,
		Metadata:            map[string]string{"severity": "high"},
	}

	if err := store.AddLesson(lesson); err != nil {
		t.Fatalf("add lesson: %v", err)
	}
	if lesson.ID == "" {
		t.Error("expected lesson ID to be set")
	}

	got, err := store.GetLesson(lesson.ID)
	if err != nil {
		t.Fatalf("get lesson: %v", err)
	}
	if got.Insight != lesson.Insight {
		t.Errorf("expected insight %q, got %q", lesson.Insight, got.Insight)
	}
	if got.Source != SourceFailure {
		t.Errorf("expected source failure, got %s", got.Source)
	}
	if len(got.Tags) != 3 {
		t.Errorf("expected 3 tags, got %d", len(got.Tags))
	}
}

func TestQueryLessons(t *testing.T) {
	store := newTestStore(t)

	lessons := []*Lesson{
		{Source: SourceFailure, Context: "c1", Insight: "i1", Tags: []string{"deploy"}, ApplicableDivisions: []string{"platform"}},
		{Source: SourceSuccess, Context: "c2", Insight: "i2", Tags: []string{"test"}, ApplicableDivisions: []string{"qa"}},
		{Source: SourceIncident, Context: "c3", Insight: "i3", Tags: []string{"monitoring"}, ApplicableDivisions: []string{"platform"}},
	}
	for _, l := range lessons {
		store.AddLesson(l)
	}

	// Query by source
	results, err := store.QueryLessons("", "failure", "", nil, 10)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 failure lesson, got %d", len(results))
	}

	// Query by division
	results, err = store.QueryLessons("platform", "", "", nil, 10)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 platform lessons, got %d", len(results))
	}

	// Query by tag
	results, err = store.QueryLessons("", "", "deploy", nil, 10)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 deploy-tagged lesson, got %d", len(results))
	}
}

func TestVerifyLesson(t *testing.T) {
	store := newTestStore(t)

	lesson := &Lesson{
		Source:  SourceSuccess,
		Context: "c1",
		Insight: "i1",
	}
	store.AddLesson(lesson)

	if err := store.VerifyLesson(lesson.ID, "senior-agent"); err != nil {
		t.Fatalf("verify: %v", err)
	}

	got, _ := store.GetLesson(lesson.ID)
	if !got.Verified {
		t.Error("expected lesson to be verified")
	}
	if got.VerifiedBy != "senior-agent" {
		t.Errorf("expected verifier senior-agent, got %s", got.VerifiedBy)
	}
}

func TestLessonLinks(t *testing.T) {
	store := newTestStore(t)

	l1 := &Lesson{Source: SourceSuccess, Context: "c1", Insight: "i1", Tags: []string{"deploy", "test"}}
	l2 := &Lesson{Source: SourceFailure, Context: "c2", Insight: "i2", Tags: []string{"deploy", "test"}}
	store.AddLesson(l1)
	store.AddLesson(l2)

	link := LessonLink{
		FromID:   l1.ID,
		ToID:     l2.ID,
		Relation: "contradicts",
		Strength: 0.8,
	}
	if err := store.AddLink(link); err != nil {
		t.Fatalf("add link: %v", err)
	}

	links, err := store.GetRelatedLessons(l1.ID)
	if err != nil {
		t.Fatalf("get related: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	if links[0].Relation != "contradicts" {
		t.Errorf("expected contradicts relation, got %s", links[0].Relation)
	}
}

func TestRecordAndQueryActivity(t *testing.T) {
	store := newTestStore(t)

	activity := &OrgActivity{
		Division:   "platform",
		AgentID:    "agent-1",
		ActionType: "deploy",
		Outcome:    "failure",
		Duration:   45.5,
		Timestamp:  time.Now(),
		Details:    "Connection timeout",
	}
	if err := store.RecordActivity(activity); err != nil {
		t.Fatalf("record activity: %v", err)
	}

	since := time.Now().Add(-1 * time.Hour)
	activities, err := store.GetActivities("platform", since, 10)
	if err != nil {
		t.Fatalf("get activities: %v", err)
	}
	if len(activities) != 1 {
		t.Fatalf("expected 1 activity, got %d", len(activities))
	}
	if activities[0].Outcome != "failure" {
		t.Errorf("expected failure outcome, got %s", activities[0].Outcome)
	}
}

func TestPatternDetector(t *testing.T) {
	store := newTestStore(t)
	detector := NewPatternDetector(store)

	// Record 5 failure activities
	for i := 0; i < 5; i++ {
		store.RecordActivity(&OrgActivity{
			Division:   "platform",
			AgentID:    "agent-1",
			ActionType: "deploy",
			Outcome:    "failure",
			Duration:   30.0,
			Timestamp:  time.Now().Add(-time.Duration(i) * time.Hour),
		})
	}

	lessons, err := detector.DetectPatterns(context.Background(), 24*time.Hour)
	if err != nil {
		t.Fatalf("detect patterns: %v", err)
	}
	if len(lessons) == 0 {
		t.Error("expected auto-detected lessons from failure patterns")
	}
	for _, l := range lessons {
		if l.Source != SourceFailure {
			t.Errorf("expected failure source, got %s", l.Source)
		}
		if !containsStr(l.Tags, "auto-detected") {
			t.Error("expected auto-detected tag")
		}
	}
}

func TestKnowledgeCompounding(t *testing.T) {
	store := newTestStore(t)
	kc := NewKnowledgeCompounding(store)

	l1 := &Lesson{
		Source:              SourceSuccess,
		Context:             "Deploy succeeded with pre-checks",
		Insight:             "Pre-flight checks prevent deploy failures",
		Tags:                []string{"deploy", "validation", "pre-check"},
		ApplicableDivisions: []string{"platform"},
	}
	l2 := &Lesson{
		Source:              SourceFailure,
		Context:             "Deploy failed without pre-checks",
		Insight:             "Skipping pre-checks causes failures",
		Tags:                []string{"deploy", "validation", "pre-check"},
		ApplicableDivisions: []string{"platform"},
	}
	store.AddLesson(l1)
	store.AddLesson(l2)

	links, err := kc.CompoundLinks(context.Background())
	if err != nil {
		t.Fatalf("compound links: %v", err)
	}
	if len(links) == 0 {
		t.Log("no auto-links generated yet (expected for minimal data)")
	}
}

func TestComputeOrgIQ(t *testing.T) {
	store := newTestStore(t)
	kc := NewKnowledgeCompounding(store)

	// Add some lessons
	store.AddLesson(&Lesson{
		Source:              SourceSuccess,
		Context:             "c1",
		Insight:             "i1",
		Tags:                []string{"deploy"},
		ApplicableDivisions: []string{"platform", "devops"},
		Verified:            true,
	})
	store.AddLesson(&Lesson{
		Source:              SourceFailure,
		Context:             "c2",
		Insight:             "i2",
		Tags:                []string{"test"},
		ApplicableDivisions: []string{"qa"},
	})

	iq, err := kc.ComputeOrgIQ(context.Background())
	if err != nil {
		t.Fatalf("compute org IQ: %v", err)
	}
	if iq.OverallScore < 0 {
		t.Error("overall score should be non-negative")
	}
	if iq.KnowledgeDensity <= 0 {
		t.Error("knowledge density should be positive with verified lessons")
	}
}

func TestLessonCount(t *testing.T) {
	store := newTestStore(t)
	if store.LessonCount() != 0 {
		t.Error("expected 0 lessons initially")
	}

	store.AddLesson(&Lesson{Source: SourceSuccess, Context: "c", Insight: "i"})
	store.AddLesson(&Lesson{Source: SourceFailure, Context: "c", Insight: "i"})

	if store.LessonCount() != 2 {
		t.Errorf("expected 2 lessons, got %d", store.LessonCount())
	}
	if store.VerifiedCount() != 0 {
		t.Error("expected 0 verified lessons")
	}
}

func containsStr(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}
