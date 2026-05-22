// Package education provides course creation, mentorship management, progression
// assessment, and knowledge externalization. It closes the gap in educational
// intelligence — enabling the Forge to teach humans, structure learning paths,
// and externalize institutional knowledge for broader consumption.
package education

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

// Course represents a structured learning course.
type Course struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Topic       string    `json:"topic"`
	Level       string    `json:"level"` // beginner, intermediate, advanced, expert
	Duration    string    `json:"duration"`
	Modules     []string  `json:"modules"`
	Prerequisites []string `json:"prerequisites"`
	Status      string    `json:"status"` // draft, published, archived
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Mentorship represents a mentor-mentee relationship.
type Mentorship struct {
	ID          string    `json:"id"`
	Mentor      string    `json:"mentor"`
	Mentee      string    `json:"mentee"`
	Topic       string    `json:"topic"`
	Goals       []string  `json:"goals"`
	Status      string    `json:"status"` // active, paused, completed, terminated
	StartDate   time.Time `json:"start_date"`
	EndDate     time.Time `json:"end_date"`
	ProgressPercent float64 `json:"progress_percent"`
}

// ProgressionLevel tracks a learner's progression through a domain.
type ProgressionLevel struct {
	ID          string    `json:"id"`
	LearnerID   string    `json:"learner_id"`
	Domain      string    `json:"domain"`
	Level       int       `json:"level"` // 1-10
	Title       string    `json:"title"`
	Skills      []string  `json:"skills"`
	AssessedAt  time.Time `json:"assessed_at"`
	NextSteps   []string  `json:"next_steps"`
}

// KnowledgeExport represents externalized knowledge.
type KnowledgeExport struct {
	ID          string    `json:"id"`
	SourceID    string    `json:"source_id"`
	Title       string    `json:"title"`
	Format      string    `json:"format"` // document, video, workshop, playbook
	Content     string    `json:"content"`
	Audience    string    `json:"audience"`
	ExportedAt  time.Time `json:"exported_at"`
	Quality     float64   `json:"quality"` // 0-1
}

// TeachingRecord logs a teaching interaction.
type TeachingRecord struct {
	ID          string    `json:"id"`
	TeacherID   string    `json:"teacher_id"`
	LearnerID   string    `json:"learner_id"`
	Topic       string    `json:"topic"`
	Method      string    `json:"method"` // lecture, hands_on, socratic, pair
	Duration    int       `json:"duration"` // minutes
	Effectiveness float64 `json:"effectiveness"` // 0-1
	RecordedAt  time.Time `json:"recorded_at"`
}

// EducationReport is a consolidated education report.
type EducationReport struct {
	GeneratedAt      time.Time        `json:"generated_at"`
	Courses          []Course         `json:"courses"`
	Mentorships      []Mentorship     `json:"mentorships"`
	ProgressionLevels []ProgressionLevel `json:"progression_levels"`
	KnowledgeExports []KnowledgeExport `json:"knowledge_exports"`
	TeachingRecords  []TeachingRecord `json:"teaching_records"`
}

// Store persists education data to a JSON file with thread safety.
type Store struct {
	mu               sync.Mutex
	filePath         string
	Courses          []Course         `json:"courses"`
	Mentorships      []Mentorship     `json:"mentorships"`
	ProgressionLevels []ProgressionLevel `json:"progression_levels"`
	KnowledgeExports []KnowledgeExport `json:"knowledge_exports"`
	TeachingRecords  []TeachingRecord `json:"teaching_records"`
}

// NewStore creates a new Store backed by the given file path.
func NewStore(filePath string) *Store {
	return &Store{filePath: filePath}
}

// Load reads data from the backing file.
func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return json.Unmarshal(data, s)
}

// Save writes data to the backing file.
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, data, 0644)
}

// CreateCourse builds a new course.
func CreateCourse(title, topic, level, duration string, modules, prerequisites []string) Course {
	return Course{
		ID:            genID("co"),
		Title:         title,
		Topic:         topic,
		Level:         level,
		Duration:      duration,
		Modules:       modules,
		Prerequisites: prerequisites,
		Status:        "draft",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
}

// StartMentorship creates a mentorship relationship.
func StartMentorship(mentor, mentee, topic string, goals []string) Mentorship {
	return Mentorship{
		ID:        genID("ms"),
		Mentor:    mentor,
		Mentee:    mentee,
		Topic:     topic,
		Goals:     goals,
		Status:    "active",
		StartDate: time.Now(),
	}
}

// AssessProgression evaluates a learner's current level in a domain.
func AssessProgression(learnerID, domain string, completedSkills, totalSkills int) ProgressionLevel {
	level := 1
	if totalSkills > 0 {
		ratio := float64(completedSkills) / float64(totalSkills)
		level = int(ratio*9) + 1
		if level > 10 {
			level = 10
		}
	}

	titles := []string{"Novice", "Beginner", "Learner", "Apprentice", "Practitioner",
		"Competent", "Proficient", "Expert", "Master", "Distinguished"}
	title := titles[0]
	if level-1 < len(titles) {
		title = titles[level-1]
	}

	var skills []string
	for i := 0; i < completedSkills; i++ {
		skills = append(skills, "skill_"+string(rune('A'+i%26)))
	}

	nextSteps := []string{"Continue practice", "Seek feedback"}
	if level < 10 {
		nextSteps = append(nextSteps, "Advance to next level")
	}

	return ProgressionLevel{
		ID:         genID("pl"),
		LearnerID:  learnerID,
		Domain:     domain,
		Level:      level,
		Title:      title,
		Skills:     skills,
		AssessedAt: time.Now(),
		NextSteps:  nextSteps,
	}
}

// ExternalizeKnowledge exports knowledge in a specified format.
func ExternalizeKnowledge(sourceID, title, format, content, audience string, quality float64) KnowledgeExport {
	return KnowledgeExport{
		ID:         genID("ke"),
		SourceID:   sourceID,
		Title:      title,
		Format:     format,
		Content:    content,
		Audience:   audience,
		ExportedAt: time.Now(),
		Quality:    quality,
	}
}

// GenerateEducationReport produces a consolidated education report.
func GenerateEducationReport(s *Store) EducationReport {
	s.mu.Lock()
	defer s.mu.Unlock()

	return EducationReport{
		GeneratedAt:      time.Now(),
		Courses:          s.Courses,
		Mentorships:      s.Mentorships,
		ProgressionLevels: s.ProgressionLevels,
		KnowledgeExports: s.KnowledgeExports,
		TeachingRecords:  s.TeachingRecords,
	}
}

func genID(prefix string) string {
	return prefix + "_" + time.Now().Format("20060102150405")
}
