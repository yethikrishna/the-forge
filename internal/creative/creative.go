// Package creative provides aesthetic judgment, brand identity modeling, creative
// leap generation, storytelling, and experience design. It closes the gap in
// creative intelligence — enabling the Forge to reason about beauty, identity,
// narrative, and user experience rather than only logic and efficiency.
package creative

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

// AestheticScore captures a subjective aesthetic assessment.
type AestheticScore struct {
	ID           string    `json:"id"`
	Subject      string    `json:"subject"`
	Score        float64   `json:"score"`         // 0-1
	Dimensions   map[string]float64 `json:"dimensions"` // harmony, contrast, balance, novelty, clarity
	AssessedAt   time.Time `json:"assessed_at"`
	Reviewer     string    `json:"reviewer"`
}

// BrandIdentity represents a brand's identity and positioning.
type BrandIdentity struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Values       []string  `json:"values"`
	Personality  string    `json:"personality"` // playful, serious, bold, minimal, warm
	VoiceTone    string    `json:"voice_tone"`
	ColorPalette []string  `json:"color_palette"`
	Tagline      string    `json:"tagline"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// CreativeLeap represents an unexpected creative insight or connection.
type CreativeLeap struct {
	ID          string    `json:"id"`
	Domain      string    `json:"domain"`
	From        string    `json:"from"`
	To          string    `json:"to"`
	Insight     string    `json:"insight"`
	Surprise    float64   `json:"surprise"` // 0-1 how unexpected
	Feasibility float64   `json:"feasibility"` // 0-1 how achievable
	CreatedAt   time.Time `json:"created_at"`
}

// Story represents a narrative structure.
type Story struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Protagonist string    `json:"protagonist"`
	Conflict    string    `json:"conflict"`
	Resolution  string    `json:"resolution"`
	Moral       string    `json:"moral"`
	Audience    string    `json:"audience"`
	CreatedAt   time.Time `json:"created_at"`
}

// ExperienceDesign captures a designed user experience.
type ExperienceDesign struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Touchpoints []string  `json:"touchpoints"`
	EmotionMap  map[string]string `json:"emotion_map"` // touchpoint -> desired emotion
	FlowScore   float64   `json:"flow_score"` // 0-1
	UpdatedAt   time.Time `json:"updated_at"`
}

// CreativeReport is a consolidated report of all creative data.
type CreativeReport struct {
	GeneratedAt      time.Time        `json:"generated_at"`
	AestheticScores  []AestheticScore `json:"aesthetic_scores"`
	BrandIdentities  []BrandIdentity  `json:"brand_identities"`
	CreativeLeaps    []CreativeLeap   `json:"creative_leaps"`
	Stories          []Story          `json:"stories"`
	ExperienceDesigns []ExperienceDesign `json:"experience_designs"`
}

// Store persists creative data to a JSON file with thread safety.
type Store struct {
	mu                sync.Mutex
	filePath          string
	AestheticScores   []AestheticScore  `json:"aesthetic_scores"`
	BrandIdentities   []BrandIdentity   `json:"brand_identities"`
	CreativeLeaps     []CreativeLeap    `json:"creative_leaps"`
	Stories           []Story           `json:"stories"`
	ExperienceDesigns []ExperienceDesign `json:"experience_designs"`
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

// JudgeAesthetics evaluates aesthetic quality across dimensions.
func JudgeAesthetics(subject, reviewer string, dims map[string]float64) AestheticScore {
	total := 0.0
	for _, v := range dims {
		total += v
	}
	score := 0.0
	if len(dims) > 0 {
		score = total / float64(len(dims))
	}

	return AestheticScore{
		ID:         genID("as"),
		Subject:    subject,
		Score:      score,
		Dimensions: dims,
		AssessedAt: time.Now(),
		Reviewer:   reviewer,
	}
}

// DevelopBrand creates a brand identity from parameters.
func DevelopBrand(name, personality, voiceTone, tagline string, values, colors []string) BrandIdentity {
	return BrandIdentity{
		ID:           genID("bi"),
		Name:         name,
		Values:       values,
		Personality:  personality,
		VoiceTone:    voiceTone,
		ColorPalette: colors,
		Tagline:      tagline,
		UpdatedAt:    time.Now(),
	}
}

// GenerateCreativeLeap produces a creative connection between two domains.
func GenerateCreativeLeap(domain, from, to, insight string, surprise, feasibility float64) CreativeLeap {
	return CreativeLeap{
		ID:          genID("cl"),
		Domain:      domain,
		From:        from,
		To:          to,
		Insight:     insight,
		Surprise:    surprise,
		Feasibility: feasibility,
		CreatedAt:   time.Now(),
	}
}

// TellStory constructs a narrative from structural elements.
func TellStory(title, protagonist, conflict, resolution, moral, audience string) Story {
	return Story{
		ID:          genID("st"),
		Title:       title,
		Protagonist: protagonist,
		Conflict:    conflict,
		Resolution:  resolution,
		Moral:       moral,
		Audience:    audience,
		CreatedAt:   time.Now(),
	}
}

// DesignExperience creates an experience design with touchpoints and emotion mapping.
func DesignExperience(name string, touchpoints []string, emotionMap map[string]string) ExperienceDesign {
	mapped := 0
	for _, tp := range touchpoints {
		if _, ok := emotionMap[tp]; ok {
			mapped++
		}
	}
	flowScore := 0.0
	if len(touchpoints) > 0 {
		flowScore = float64(mapped) / float64(len(touchpoints))
	}

	return ExperienceDesign{
		ID:          genID("ed"),
		Name:        name,
		Touchpoints: touchpoints,
		EmotionMap:  emotionMap,
		FlowScore:   flowScore,
		UpdatedAt:   time.Now(),
	}
}

// GenerateCreativeReport produces a consolidated creative report.
func GenerateCreativeReport(s *Store) CreativeReport {
	s.mu.Lock()
	defer s.mu.Unlock()

	return CreativeReport{
		GeneratedAt:       time.Now(),
		AestheticScores:   s.AestheticScores,
		BrandIdentities:   s.BrandIdentities,
		CreativeLeaps:     s.CreativeLeaps,
		Stories:           s.Stories,
		ExperienceDesigns: s.ExperienceDesigns,
	}
}

func genID(prefix string) string {
	return prefix + "_" + time.Now().Format("20060102150405")
}
