// Package customer provides human-vs-AI transparency, consistency enforcement,
// and single-source-of-truth management for customer interactions. It closes
// the gap where customers receive inconsistent answers from different channels,
// can't tell if they're talking to a human or AI, or get contradictory
// information across touchpoints. Every interaction is recorded, every answer
// checked against the authoritative source, and every response is transparent
// about its origin.
package customer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// InteractionChannel is the communication channel used.
type InteractionChannel string

const (
	ChannelChat     InteractionChannel = "chat"
	ChannelEmail    InteractionChannel = "email"
	ChannelPhone    InteractionChannel = "phone"
	ChannelSocial   InteractionChannel = "social"
	ChannelInApp    InteractionChannel = "in_app"
	ChannelAPI      InteractionChannel = "api"
)

// InteractionOrigin indicates whether the responder was human or AI.
type InteractionOrigin string

const (
	OriginHuman InteractionOrigin = "human"
	OriginAI    InteractionOrigin = "ai"
	OriginMixed InteractionOrigin = "mixed"
)

// ConsistencyStatus indicates whether an answer is consistent.
type ConsistencyStatus string

const (
	Consistent      ConsistencyStatus = "consistent"
	Inconsistent    ConsistencyStatus = "inconsistent"
	Unverified      ConsistencyStatus = "unverified"
	Overridden      ConsistencyStatus = "overridden"
)

// CustomerInteraction records a single customer interaction.
type CustomerInteraction struct {
	ID           string             `json:"id"`
	CustomerID   string             `json:"customer_id"`
	Channel      InteractionChannel `json:"channel"`
	Topic        string             `json:"topic"`
	Question     string             `json:"question"`
	Answer       string             `json:"answer"`
	Origin       InteractionOrigin  `json:"origin"`
	AgentID      string             `json:"agent_id,omitempty"`  // who/what responded
	Consistent   ConsistencyStatus  `json:"consistent"`
	TrustScore   float64            `json:"trust_score"` // 0-1, how confident in the answer
	CreatedAt    time.Time          `json:"created_at"`
}

// ResponsePolicy defines what to reveal about AI involvement.
type ResponsePolicy struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	Channel         InteractionChannel `json:"channel"`
	RevealAI        bool    `json:"reveal_ai"`         // must we disclose AI origin?
	ProactiveDisclose bool  `json:"proactive_disclose"` // volunteer the info vs only if asked
	SignaturePhrase string  `json:"signature_phrase,omitempty"` // e.g., "This response was AI-generated"
	Active          bool    `json:"active"`
	CreatedAt       time.Time `json:"created_at"`
}

// ConsistencyCheck represents the result of checking an answer against the SSOT.
type ConsistencyCheck struct {
	ID              string            `json:"id"`
	InteractionID   string            `json:"interaction_id"`
	Topic           string            `json:"topic"`
	Answer          string            `json:"answer"`
	Authoritative   string            `json:"authoritative"` // the SSOT answer
	Status          ConsistencyStatus `json:"status"`
	Similarity      float64           `json:"similarity"` // 0-1
	Discrepancies   []string          `json:"discrepancies,omitempty"`
	CheckedAt       time.Time         `json:"checked_at"`
}

// SingleSourceOfTruth is an authoritative answer for a topic.
type SingleSourceOfTruth struct {
	ID          string    `json:"id"`
	Topic       string    `json:"topic"`
	Answer      string    `json:"answer"`
	Source      string    `json:"source"` // "docs", "product", "legal", "exec"
	VerifiedBy  string    `json:"verified_by,omitempty"`
	Version      int       `json:"version"`
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CustomerManager manages customer interactions, consistency, and transparency.
type CustomerManager struct {
	mu          sync.RWMutex
	interactions map[string]*CustomerInteraction
	policies    map[string]*ResponsePolicy
	checks      map[string]*ConsistencyCheck
	ssots       map[string]*SingleSourceOfTruth
	path        string
}

// NewCustomerManager creates a new CustomerManager store.
func NewCustomerManager(persistPath string) *CustomerManager {
	cm := &CustomerManager{
		interactions: make(map[string]*CustomerInteraction),
		policies:     make(map[string]*ResponsePolicy),
		checks:       make(map[string]*ConsistencyCheck),
		ssots:        make(map[string]*SingleSourceOfTruth),
		path:         persistPath,
	}
	cm.load()
	return cm
}

// --- Interactions ---

// RecordInteraction records a customer interaction.
func (cm *CustomerManager) RecordInteraction(customerID string, channel InteractionChannel, topic, question, answer string, origin InteractionOrigin, agentID string) (*CustomerInteraction, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Check consistency against SSOT
	consistent := Unverified
	trustScore := 0.5
	if ssot, ok := cm.findSSOT(topic); ok {
		similarity := textSimilarity(answer, ssot.Answer)
		if similarity >= 0.8 {
			consistent = Consistent
			trustScore = similarity
		} else if similarity >= 0.5 {
			consistent = Unverified
			trustScore = similarity
		} else {
			consistent = Inconsistent
			trustScore = similarity
		}
	}

	interaction := &CustomerInteraction{
		ID:         genID("int"),
		CustomerID: customerID,
		Channel:    channel,
		Topic:      topic,
		Question:   question,
		Answer:     answer,
		Origin:     origin,
		AgentID:    agentID,
		Consistent: consistent,
		TrustScore: trustScore,
		CreatedAt:  time.Now().UTC(),
	}
	cm.interactions[interaction.ID] = interaction
	cm.persist()
	return interaction, nil
}

// ListInteractions returns interactions for a customer.
func (cm *CustomerManager) ListInteractions(customerID string) []*CustomerInteraction {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	var result []*CustomerInteraction
	for _, i := range cm.interactions {
		if i.CustomerID == customerID {
			result = append(result, i)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.After(result[j].CreatedAt) })
	return result
}

// --- Consistency ---

// CheckConsistency explicitly checks an answer against the SSOT.
func (cm *CustomerManager) CheckConsistency(interactionID string) (*ConsistencyCheck, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	interaction, ok := cm.interactions[interactionID]
	if !ok {
		return nil, fmt.Errorf("interaction %s not found", interactionID)
	}

	ssot, hasSSOT := cm.findSSOT(interaction.Topic)
	if !hasSSOT {
		return &ConsistencyCheck{
			ID:            genID("chk"),
			InteractionID: interactionID,
			Topic:         interaction.Topic,
			Answer:        interaction.Answer,
			Authoritative: "",
			Status:        Unverified,
			Similarity:    0,
			Discrepancies: []string{"No authoritative source found for topic"},
			CheckedAt:     time.Now().UTC(),
		}, nil
	}

	similarity := textSimilarity(interaction.Answer, ssot.Answer)
	status := Consistent
	var discrepancies []string
	if similarity < 0.8 {
		status = Inconsistent
		discrepancies = append(discrepancies, fmt.Sprintf("Answer similarity %.0f%% below 80%% threshold", similarity*100))
	}
	if similarity < 0.5 {
		discrepancies = append(discrepancies, "Significant divergence from authoritative answer")
	}

	check := &ConsistencyCheck{
		ID:            genID("chk"),
		InteractionID: interactionID,
		Topic:         interaction.Topic,
		Answer:        interaction.Answer,
		Authoritative: ssot.Answer,
		Status:        status,
		Similarity:    similarity,
		Discrepancies: discrepancies,
		CheckedAt:     time.Now().UTC(),
	}
	cm.checks[check.ID] = check
	cm.persist()
	return check, nil
}

// GetAuthoritativeAnswer returns the SSOT answer for a topic.
func (cm *CustomerManager) GetAuthoritativeAnswer(topic string) (*SingleSourceOfTruth, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	ssot, ok := cm.findSSOT(topic)
	if !ok {
		return nil, fmt.Errorf("no authoritative answer for topic %q", topic)
	}
	return ssot, nil
}

// --- SSOT Management ---

// SetAuthoritativeAnswer creates or updates the SSOT for a topic.
func (cm *CustomerManager) SetAuthoritativeAnswer(topic, answer, source, verifiedBy string) (*SingleSourceOfTruth, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Check if one already exists
	for _, s := range cm.ssots {
		if s.Topic == topic && s.Active {
			s.Answer = answer
			s.Source = source
			s.VerifiedBy = verifiedBy
			s.Version++
			s.UpdatedAt = time.Now().UTC()
			cm.persist()
			return s, nil
		}
	}

	ssot := &SingleSourceOfTruth{
		ID:         genID("ssot"),
		Topic:      topic,
		Answer:     answer,
		Source:     source,
		VerifiedBy: verifiedBy,
		Version:    1,
		Active:     true,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	cm.ssots[ssot.ID] = ssot
	cm.persist()
	return ssot, nil
}

// --- Transparency Policies ---

// SetResponsePolicy creates or updates a response policy for a channel.
func (cm *CustomerManager) SetResponsePolicy(name string, channel InteractionChannel, revealAI, proactiveDisclose bool, signaturePhrase string) (*ResponsePolicy, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	p := &ResponsePolicy{
		ID:                genID("pol"),
		Name:              name,
		Channel:           channel,
		RevealAI:          revealAI,
		ProactiveDisclose: proactiveDisclose,
		SignaturePhrase:   signaturePhrase,
		Active:            true,
		CreatedAt:         time.Now().UTC(),
	}
	cm.policies[p.ID] = p
	cm.persist()
	return p, nil
}

// GetResponsePolicy returns the active policy for a channel.
func (cm *CustomerManager) GetResponsePolicy(channel InteractionChannel) (*ResponsePolicy, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	for _, p := range cm.policies {
		if p.Channel == channel && p.Active {
			return p, nil
		}
	}
	return nil, fmt.Errorf("no active policy for channel %s", channel)
}

// --- Reports ---

// GenerateTransparencyReport produces a report on AI transparency and consistency.
func (cm *CustomerManager) GenerateTransparencyReport() map[string]interface{} {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	total, consistent, inconsistent, unverified := 0, 0, 0, 0
	humanCount, aiCount, mixedCount := 0, 0, 0
	for _, i := range cm.interactions {
		total++
		switch i.Consistent {
		case Consistent:
			consistent++
		case Inconsistent:
			inconsistent++
		case Unverified:
			unverified++
		}
		switch i.Origin {
		case OriginHuman:
			humanCount++
		case OriginAI:
			aiCount++
		case OriginMixed:
			mixedCount++
		}
	}

	consistencyRate := 0.0
	if total > 0 {
		consistencyRate = float64(consistent) / float64(total) * 100
	}

	return map[string]interface{}{
		"total_interactions": total,
		"consistent":         consistent,
		"inconsistent":       inconsistent,
		"unverified":         unverified,
		"consistency_rate":   consistencyRate,
		"human_responses":    humanCount,
		"ai_responses":       aiCount,
		"mixed_responses":    mixedCount,
		"ssot_entries":       len(cm.ssots),
		"policies":           len(cm.policies),
		"generated_at":       time.Now().UTC(),
	}
}

// --- Helpers ---

func (cm *CustomerManager) findSSOT(topic string) (*SingleSourceOfTruth, bool) {
	for _, s := range cm.ssots {
		if s.Topic == topic && s.Active {
			return s, true
		}
	}
	return nil, false
}

// textSimilarity computes a simple token-overlap similarity between two strings.
func textSimilarity(a, b string) float64 {
	tokensA := tokenize(strings.ToLower(a))
	tokensB := tokenize(strings.ToLower(b))
	if len(tokensA) == 0 && len(tokensB) == 0 {
		return 1.0
	}
	if len(tokensA) == 0 || len(tokensB) == 0 {
		return 0.0
	}

	setA := make(map[string]bool)
	for _, t := range tokensA {
		setA[t] = true
	}
	overlap := 0
	for _, t := range tokensB {
		if setA[t] {
			overlap++
		}
	}

	// Jaccard-like: overlap / union
	union := len(tokensA) + len(tokensB) - overlap
	if union == 0 {
		return 1.0
	}
	return float64(overlap) / float64(union)
}

func tokenize(s string) []string {
	return strings.Fields(s)
}

func (cm *CustomerManager) persist() {
	if cm.path == "" {
		return
	}
	data := struct {
		Interactions map[string]*CustomerInteraction `json:"interactions"`
		Policies     map[string]*ResponsePolicy      `json:"policies"`
		Checks       map[string]*ConsistencyCheck     `json:"checks"`
		SSOTs        map[string]*SingleSourceOfTruth  `json:"ssots"`
	}{cm.interactions, cm.policies, cm.checks, cm.ssots}
	raw, _ := json.MarshalIndent(data, "", "  ")
	os.MkdirAll(filepath.Dir(cm.path), 0755)
	os.WriteFile(cm.path, raw, 0644)
}

func (cm *CustomerManager) load() {
	if cm.path == "" {
		return
	}
	raw, err := os.ReadFile(cm.path)
	if err != nil {
		return
	}
	var data struct {
		Interactions map[string]*CustomerInteraction `json:"interactions"`
		Policies     map[string]*ResponsePolicy      `json:"policies"`
		Checks       map[string]*ConsistencyCheck     `json:"checks"`
		SSOTs        map[string]*SingleSourceOfTruth  `json:"ssots"`
	}
	if json.Unmarshal(raw, &data) == nil {
		if data.Interactions != nil {
			cm.interactions = data.Interactions
		}
		if data.Policies != nil {
			cm.policies = data.Policies
		}
		if data.Checks != nil {
			cm.checks = data.Checks
		}
		if data.SSOTs != nil {
			cm.ssots = data.SSOTs
		}
	}
}

func genID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}
