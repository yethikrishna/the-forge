// Package orglearn implements organizational learning — the org itself gets smarter
// over time as institutional knowledge compounds through lessons, pattern detection,
// and knowledge graph relationships.
package orglearn

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// LessonSource indicates where a lesson originated.
type LessonSource string

const (
	SourceFailure    LessonSource = "failure"
	SourceSuccess    LessonSource = "success"
	SourceExperiment LessonSource = "experiment"
	SourceIncident   LessonSource = "incident"
)

// Lesson represents a piece of organizational knowledge.
type Lesson struct {
	ID                  string            `json:"id"`
	Source              LessonSource      `json:"source"`
	Context             string            `json:"context"`
	Insight             string            `json:"insight"`
	Tags                []string          `json:"tags"`
	ApplicableDivisions []string          `json:"applicable_divisions"`
	Verified            bool              `json:"verified"`
	VerifiedBy          string            `json:"verified_by,omitempty"`
	CreatedAt           time.Time         `json:"created_at"`
	UpdatedAt           time.Time         `json:"updated_at"`
	RelatedLessons      []string          `json:"related_lessons,omitempty"`
	Metadata            map[string]string `json:"metadata,omitempty"`
}

// OrgActivity represents an observable action in the organization.
type OrgActivity struct {
	ID         string    `json:"id"`
	Division   string    `json:"division"`
	AgentID    string    `json:"agent_id"`
	ActionType string    `json:"action_type"`
	Outcome    string    `json:"outcome"` // success, failure, partial
	Duration   float64   `json:"duration_seconds"`
	Timestamp  time.Time `json:"timestamp"`
	Details    string    `json:"details,omitempty"`
}

// LessonLink represents a relationship between two lessons.
type LessonLink struct {
	FromID    string    `json:"from_id"`
	ToID      string    `json:"to_id"`
	Relation  string    `json:"relation"` // depends_on, contradicts, refines, relates_to
	Strength  float64   `json:"strength"` // 0-1
	CreatedAt time.Time `json:"created_at"`
}

// OrgIQ represents the composite organizational intelligence score.
type OrgIQ struct {
	LearningVelocity float64 `json:"learning_velocity"` // lessons per week
	KnowledgeDensity float64 `json:"knowledge_density"` // verified lessons / total lessons
	CoverageScore    float64 `json:"coverage_score"`    // divisions with lessons / total divisions
	CompoundFactor   float64 `json:"compound_factor"`   // avg related lessons per lesson
	OverallScore     float64 `json:"overall_score"`     // weighted composite
	ComputedAt       time.Time `json:"computed_at"`
}

// LearningStore manages lessons with SQLite persistence.
type LearningStore struct {
	db *sql.DB
	mu  sync.RWMutex
}

// NewLearningStore creates a new learning store backed by SQLite.
func NewLearningStore(dbPath string) (*LearningStore, error) {
	os.MkdirAll(filepath.Dir(dbPath), 0o755)
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	store := &LearningStore{db: db}
	if err := store.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return store, nil
}

// migrate creates the database schema.
func (ls *LearningStore) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS lessons (
		id TEXT PRIMARY KEY,
		source TEXT NOT NULL,
		context TEXT NOT NULL,
		insight TEXT NOT NULL,
		tags TEXT DEFAULT '[]',
		applicable_divisions TEXT DEFAULT '[]',
		verified INTEGER DEFAULT 0,
		verified_by TEXT DEFAULT '',
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		related_lessons TEXT DEFAULT '[]',
		metadata TEXT DEFAULT '{}'
	);
	CREATE TABLE IF NOT EXISTS lesson_links (
		from_id TEXT NOT NULL,
		to_id TEXT NOT NULL,
		relation TEXT NOT NULL,
		strength REAL DEFAULT 0.5,
		created_at DATETIME NOT NULL,
		PRIMARY KEY (from_id, to_id, relation)
	);
	CREATE TABLE IF NOT EXISTS org_activities (
		id TEXT PRIMARY KEY,
		division TEXT NOT NULL,
		agent_id TEXT NOT NULL,
		action_type TEXT NOT NULL,
		outcome TEXT NOT NULL,
		duration_seconds REAL DEFAULT 0,
		timestamp DATETIME NOT NULL,
		details TEXT DEFAULT ''
	);
	CREATE INDEX IF NOT EXISTS idx_lessons_source ON lessons(source);
	CREATE INDEX IF NOT EXISTS idx_lessons_verified ON lessons(verified);
	CREATE INDEX IF NOT EXISTS idx_activities_division ON org_activities(division);
	CREATE INDEX IF NOT EXISTS idx_activities_timestamp ON org_activities(timestamp);`
	_, err := ls.db.Exec(schema)
	return err
}

// Close closes the database connection.
func (ls *LearningStore) Close() error {
	return ls.db.Close()
}

// AddLesson stores a new lesson.
func (ls *LearningStore) AddLesson(lesson *Lesson) error {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	if lesson.ID == "" {
		lesson.ID = fmt.Sprintf("lesson-%d", time.Now().UnixNano())
	}
	now := time.Now()
	lesson.CreatedAt = now
	lesson.UpdatedAt = now

	tags, _ := json.Marshal(lesson.Tags)
	divisions, _ := json.Marshal(lesson.ApplicableDivisions)
	related, _ := json.Marshal(lesson.RelatedLessons)
	metadata, _ := json.Marshal(lesson.Metadata)

	_, err := ls.db.Exec(`
		INSERT INTO lessons (id, source, context, insight, tags, applicable_divisions,
			verified, verified_by, created_at, updated_at, related_lessons, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		lesson.ID, lesson.Source, lesson.Context, lesson.Insight,
		string(tags), string(divisions),
		lesson.Verified, lesson.VerifiedBy,
		lesson.CreatedAt, lesson.UpdatedAt,
		string(related), string(metadata))
	return err
}

// GetLesson retrieves a lesson by ID.
func (ls *LearningStore) GetLesson(id string) (*Lesson, error) {
	ls.mu.RLock()
	defer ls.mu.RUnlock()

	row := ls.db.QueryRow(`
		SELECT id, source, context, insight, tags, applicable_divisions,
			verified, verified_by, created_at, updated_at, related_lessons, metadata
		FROM lessons WHERE id = ?`, id)

	return ls.scanLesson(row)
}

// QueryLessons searches lessons by various criteria.
func (ls *LearningStore) QueryLessons(division, source, tag string, verified *bool, limit int) ([]*Lesson, error) {
	ls.mu.RLock()
	defer ls.mu.RUnlock()

	query := `SELECT id, source, context, insight, tags, applicable_divisions,
		verified, verified_by, created_at, updated_at, related_lessons, metadata
		FROM lessons WHERE 1=1`
	args := []interface{}{}

	if division != "" {
		query += ` AND applicable_divisions LIKE ?`
		args = append(args, "%"+division+"%")
	}
	if source != "" {
		query += ` AND source = ?`
		args = append(args, source)
	}
	if tag != "" {
		query += ` AND tags LIKE ?`
		args = append(args, "%"+tag+"%")
	}
	if verified != nil {
		query += ` AND verified = ?`
		args = append(args, *verified)
	}
	query += ` ORDER BY updated_at DESC`
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}

	rows, err := ls.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lessons []*Lesson
	for rows.Next() {
		lesson, err := ls.scanLessonFromRows(rows)
		if err != nil {
			return nil, err
		}
		lessons = append(lessons, lesson)
	}
	return lessons, nil
}

// VerifyLesson marks a lesson as verified.
func (ls *LearningStore) VerifyLesson(id, verifier string) error {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	_, err := ls.db.Exec(`
		UPDATE lessons SET verified = 1, verified_by = ?, updated_at = ? WHERE id = ?`,
		verifier, time.Now(), id)
	return err
}

// AddLink creates a relationship between two lessons.
func (ls *LearningStore) AddLink(link LessonLink) error {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	if link.CreatedAt.IsZero() {
		link.CreatedAt = time.Now()
	}
	_, err := ls.db.Exec(`
		INSERT OR REPLACE INTO lesson_links (from_id, to_id, relation, strength, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		link.FromID, link.ToID, link.Relation, link.Strength, link.CreatedAt)
	return err
}

// GetRelatedLessons finds lessons related to a given lesson.
func (ls *LearningStore) GetRelatedLessons(id string) ([]LessonLink, error) {
	ls.mu.RLock()
	defer ls.mu.RUnlock()

	rows, err := ls.db.Query(`
		SELECT from_id, to_id, relation, strength, created_at
		FROM lesson_links WHERE from_id = ? OR to_id = ?`, id, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []LessonLink
	for rows.Next() {
		var link LessonLink
		if err := rows.Scan(&link.FromID, &link.ToID, &link.Relation, &link.Strength, &link.CreatedAt); err != nil {
			return nil, err
		}
		links = append(links, link)
	}
	return links, nil
}

// RecordActivity stores an organizational activity for pattern detection.
func (ls *LearningStore) RecordActivity(activity *OrgActivity) error {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	if activity.ID == "" {
		activity.ID = fmt.Sprintf("act-%d", time.Now().UnixNano())
	}
	_, err := ls.db.Exec(`
		INSERT INTO org_activities (id, division, agent_id, action_type, outcome,
			duration_seconds, timestamp, details)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		activity.ID, activity.Division, activity.AgentID, activity.ActionType,
		activity.Outcome, activity.Duration, activity.Timestamp, activity.Details)
	return err
}

// GetActivities retrieves recent activities.
func (ls *LearningStore) GetActivities(division string, since time.Time, limit int) ([]*OrgActivity, error) {
	ls.mu.RLock()
	defer ls.mu.RUnlock()

	query := `SELECT id, division, agent_id, action_type, outcome,
		duration_seconds, timestamp, details
		FROM org_activities WHERE timestamp >= ?`
	args := []interface{}{since}

	if division != "" {
		query += ` AND division = ?`
		args = append(args, division)
	}
	query += ` ORDER BY timestamp DESC`
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}

	rows, err := ls.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var activities []*OrgActivity
	for rows.Next() {
		var a OrgActivity
		if err := rows.Scan(&a.ID, &a.Division, &a.AgentID, &a.ActionType,
			&a.Outcome, &a.Duration, &a.Timestamp, &a.Details); err != nil {
			return nil, err
		}
		activities = append(activities, &a)
	}
	return activities, nil
}

// LessonCount returns the total number of lessons.
func (ls *LearningStore) LessonCount() int {
	ls.mu.RLock()
	defer ls.mu.RUnlock()
	var count int
	ls.db.QueryRow("SELECT COUNT(*) FROM lessons").Scan(&count)
	return count
}

// VerifiedCount returns the number of verified lessons.
func (ls *LearningStore) VerifiedCount() int {
	ls.mu.RLock()
	defer ls.mu.RUnlock()
	var count int
	ls.db.QueryRow("SELECT COUNT(*) FROM lessons WHERE verified = 1").Scan(&count)
	return count
}

func (ls *LearningStore) scanLesson(row *sql.Row) (*Lesson, error) {
	var l Lesson
	var tags, divisions, related, metadata string
	err := row.Scan(&l.ID, &l.Source, &l.Context, &l.Insight,
		&tags, &divisions, &l.Verified, &l.VerifiedBy,
		&l.CreatedAt, &l.UpdatedAt, &related, &metadata)
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(tags), &l.Tags)
	json.Unmarshal([]byte(divisions), &l.ApplicableDivisions)
	json.Unmarshal([]byte(related), &l.RelatedLessons)
	json.Unmarshal([]byte(metadata), &l.Metadata)
	return &l, nil
}

func (ls *LearningStore) scanLessonFromRows(rows *sql.Rows) (*Lesson, error) {
	var l Lesson
	var tags, divisions, related, metadata string
	err := rows.Scan(&l.ID, &l.Source, &l.Context, &l.Insight,
		&tags, &divisions, &l.Verified, &l.VerifiedBy,
		&l.CreatedAt, &l.UpdatedAt, &related, &metadata)
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(tags), &l.Tags)
	json.Unmarshal([]byte(divisions), &l.ApplicableDivisions)
	json.Unmarshal([]byte(related), &l.RelatedLessons)
	json.Unmarshal([]byte(metadata), &l.Metadata)
	return &l, nil
}

// PatternDetector watches org activity and auto-creates lessons from recurring patterns.
type PatternDetector struct {
	store *LearningStore
}

// NewPatternDetector creates a new pattern detector.
func NewPatternDetector(store *LearningStore) *PatternDetector {
	return &PatternDetector{store: store}
}

// DetectPatterns scans recent activities and creates lessons from patterns.
func (pd *PatternDetector) DetectPatterns(ctx context.Context, lookback time.Duration) ([]*Lesson, error) {
	since := time.Now().Add(-lookback)
	activities, err := pd.store.GetActivities("", since, 1000)
	if err != nil {
		return nil, err
	}

	var lessons []*Lesson

	// Group by action_type + outcome
	outcomeCounts := make(map[string]int)
	actionOutcomeDivs := make(map[string]map[string]int)

	for _, a := range activities {
		key := fmt.Sprintf("%s:%s", a.ActionType, a.Outcome)
		outcomeCounts[key]++
		if actionOutcomeDivs[key] == nil {
			actionOutcomeDivs[key] = make(map[string]int)
		}
		actionOutcomeDivs[key][a.Division]++
	}

	// Create lessons from failure patterns
	for key, count := range outcomeCounts {
		parts := strings.SplitN(key, ":", 2)
		actionType, outcome := parts[0], parts[1]
		if outcome == "failure" && count >= 3 {
			divs := make([]string, 0, len(actionOutcomeDivs[key]))
			for d := range actionOutcomeDivs[key] {
				divs = append(divs, d)
			}

			lesson := &Lesson{
				Source:              SourceFailure,
				Context:             fmt.Sprintf("Repeated failures in %s actions (%d occurrences)", actionType, count),
				Insight:             fmt.Sprintf("%s actions have a systematic failure pattern across divisions", actionType),
				Tags:                []string{actionType, "failure-pattern", "auto-detected"},
				ApplicableDivisions: divs,
				Verified:            false,
				Metadata:            map[string]string{"occurrence_count": fmt.Sprintf("%d", count)},
			}
			if err := pd.store.AddLesson(lesson); err == nil {
				lessons = append(lessons, lesson)
			}
		}
	}

	return lessons, nil
}

// KnowledgeCompounding links related lessons and surfaces contradictions.
type KnowledgeCompounding struct {
	store *LearningStore
}

// NewKnowledgeCompounding creates a new knowledge compounding engine.
func NewKnowledgeCompounding(store *LearningStore) *KnowledgeCompounding {
	return &KnowledgeCompounding{store: store}
}

// CompoundLinks finds and creates relationships between lessons.
func (kc *KnowledgeCompounding) CompoundLinks(ctx context.Context) ([]LessonLink, error) {
	lessons, err := kc.store.QueryLessons("", "", "", nil, 0)
	if err != nil {
		return nil, err
	}

	var links []LessonLink

	for i := 0; i < len(lessons); i++ {
		for j := i + 1; j < len(lessons); j++ {
			a, b := lessons[i], lessons[j]

			// Check for shared divisions
			sharedDivs := sharedElements(a.ApplicableDivisions, b.ApplicableDivisions)
			sharedTags := sharedElements(a.Tags, b.Tags)

			// Check for contradictions
			if a.Source == SourceSuccess && b.Source == SourceFailure && len(sharedDivs) > 0 {
				strength := math.Min(float64(len(sharedDivs)+len(sharedTags))/5.0, 1.0)
				link := LessonLink{
					FromID:    a.ID,
					ToID:      b.ID,
					Relation:  "contradicts",
					Strength:  strength,
					CreatedAt: time.Now(),
				}
				kc.store.AddLink(link)
				links = append(links, link)
			}

			// Check for refinement opportunities
			if len(sharedTags) >= 2 && a.Source == b.Source {
				strength := math.Min(float64(len(sharedTags))/5.0, 1.0)
				link := LessonLink{
					FromID:    a.ID,
					ToID:      b.ID,
					Relation:  "refines",
					Strength:  strength,
					CreatedAt: time.Now(),
				}
				kc.store.AddLink(link)
				links = append(links, link)
			}
		}
	}

	return links, nil
}

// ComputeOrgIQ calculates the organizational intelligence quotient.
func (kc *KnowledgeCompounding) ComputeOrgIQ(ctx context.Context) (*OrgIQ, error) {
	totalLessons := kc.store.LessonCount()
	verifiedLessons := kc.store.VerifiedCount()

	// Learning velocity: lessons created in the last 7 days
	recentLessons, err := kc.store.QueryLessons("", "", "", nil, 0)
	if err != nil {
		return nil, err
	}
	weekAgo := time.Now().Add(-7 * 24 * time.Hour)
	recentCount := 0
	divisionsWithLessons := make(map[string]bool)
	for _, l := range recentLessons {
		if l.CreatedAt.After(weekAgo) {
			recentCount++
		}
		for _, d := range l.ApplicableDivisions {
			divisionsWithLessons[d] = true
		}
	}

	learningVelocity := float64(recentCount)
	knowledgeDensity := 0.0
	if totalLessons > 0 {
		knowledgeDensity = float64(verifiedLessons) / float64(totalLessons)
	}

	// Get link counts for compound factor
	links, _ := kc.store.GetRelatedLessons("")
	compoundFactor := 0.0
	if totalLessons > 0 {
		compoundFactor = float64(len(links)) / float64(totalLessons)
	}

	// Coverage: assume at least 5 divisions as baseline
	totalDivisions := math.Max(float64(len(divisionsWithLessons)), 5)
	coverageScore := float64(len(divisionsWithLessons)) / totalDivisions

	overallScore := learningVelocity*0.25 + knowledgeDensity*100*0.3 + coverageScore*100*0.25 + compoundFactor*10*0.2

	return &OrgIQ{
		LearningVelocity: learningVelocity,
		KnowledgeDensity: knowledgeDensity,
		CoverageScore:    coverageScore,
		CompoundFactor:   compoundFactor,
		OverallScore:     overallScore,
		ComputedAt:       time.Now(),
	}, nil
}

func sharedElements(a, b []string) []string {
	set := make(map[string]bool)
	for _, s := range a {
		set[s] = true
	}
	var shared []string
	for _, s := range b {
		if set[s] {
			shared = append(shared, s)
		}
	}
	sort.Strings(shared)
	return shared
}
