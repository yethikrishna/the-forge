// Package suna provides a skill marketplace for Forge agents.
// Agents can publish skills they develop and consume skills from other
// Forge orgs — creating a marketplace of AI capabilities.
package suna

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// MarketplaceSkill represents a skill listed in the marketplace.
type MarketplaceSkill struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	Category      string    `json:"category"`
	Version       string    `json:"version"`
	Author        string    `json:"author"`
	OrgID         string    `json:"org_id"`        // publisher's Forge org
	Downloads     int       `json:"downloads"`
	Rating        float64   `json:"rating"`
	Reviews       int       `json:"reviews"`
	Price         float64   `json:"price"`         // 0 = free
	Verified      bool      `json:"verified"`      // verified by Forge team
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	Tags          []string  `json:"tags"`
	Documentation string    `json:"documentation"`
}

// PublishRequest is a request to publish a skill to the marketplace.
type PublishRequest struct {
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Category      string   `json:"category"`
	Version       string   `json:"version"`
	Tags          []string `json:"tags"`
	Documentation string   `json:"documentation"`
	Price         float64  `json:"price"`
	SourceURL     string   `json:"source_url"`
}

// Review represents a user review of a marketplace skill.
type Review struct {
	ID        string    `json:"id"`
	SkillID   string    `json:"skill_id"`
	AuthorID  string    `json:"author_id"`
	Rating    float64   `json:"rating"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"created_at"`
}

// Marketplace manages the skill marketplace.
type Marketplace struct {
	bridge *Bridge
	mu     sync.RWMutex
}

// NewMarketplace creates a new marketplace manager.
func NewMarketplace(bridge *Bridge) *Marketplace {
	return &Marketplace{bridge: bridge}
}

// Browse returns skills in the marketplace, optionally filtered.
func (m *Marketplace) Browse(ctx context.Context, opts BrowseOpts) ([]*MarketplaceSkill, error) {
	path := "/api/marketplace"
	params := []string{}
	if opts.Category != "" {
		params = append(params, "category="+opts.Category)
	}
	if opts.Query != "" {
		params = append(params, "q="+opts.Query)
	}
	if opts.Sort != "" {
		params = append(params, "sort="+opts.Sort)
	}
	if opts.Limit > 0 {
		params = append(params, fmt.Sprintf("limit=%d", opts.Limit))
	}
	if len(params) > 0 {
		path += "?" + params[0]
		for _, p := range params[1:] {
			path += "&" + p
		}
	}

	var skills []*MarketplaceSkill
	if err := m.bridge.GetJSON(ctx, path, &skills); err != nil {
		return nil, fmt.Errorf("browse marketplace: %w", err)
	}
	return skills, nil
}

// BrowseOpts filters for marketplace browsing.
type BrowseOpts struct {
	Category string `json:"category"`
	Query    string `json:"query"`
	Sort     string `json:"sort"` // popular, newest, rating
	Limit    int    `json:"limit"`
}

// Get returns details for a specific marketplace skill.
func (m *Marketplace) Get(ctx context.Context, id string) (*MarketplaceSkill, error) {
	var skill MarketplaceSkill
	if err := m.bridge.GetJSON(ctx, "/api/marketplace/"+id, &skill); err != nil {
		return nil, fmt.Errorf("get marketplace skill %s: %w", id, err)
	}
	return &skill, nil
}

// Publish submits a skill to the marketplace.
func (m *Marketplace) Publish(ctx context.Context, req PublishRequest) (*MarketplaceSkill, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("skill name is required")
	}
	if req.Description == "" {
		return nil, fmt.Errorf("description is required")
	}

	var skill MarketplaceSkill
	if err := m.bridge.PostJSON(ctx, "/api/marketplace/publish", req, &skill); err != nil {
		return nil, fmt.Errorf("publish skill: %w", err)
	}
	return &skill, nil
}

// Install installs a skill from the marketplace.
func (m *Marketplace) Install(ctx context.Context, skillID string) error {
	if err := m.bridge.PostJSON(ctx, "/api/marketplace/"+skillID+"/install", nil, nil); err != nil {
		return fmt.Errorf("install marketplace skill %s: %w", skillID, err)
	}
	return nil
}

// Review submits a review for a marketplace skill.
func (m *Marketplace) Review(ctx context.Context, skillID string, rating float64, comment string) error {
	if rating < 1 || rating > 5 {
		return fmt.Errorf("rating must be between 1 and 5")
	}
	payload := map[string]interface{}{
		"skillId": skillID,
		"rating":  rating,
		"comment": comment,
	}
	return m.bridge.PostJSON(ctx, "/api/marketplace/"+skillID+"/review", payload, nil)
}

// ListReviews returns reviews for a marketplace skill.
func (m *Marketplace) ListReviews(ctx context.Context, skillID string) ([]*Review, error) {
	var reviews []*Review
	path := fmt.Sprintf("/api/marketplace/%s/reviews", skillID)
	if err := m.bridge.GetJSON(ctx, path, &reviews); err != nil {
		return nil, fmt.Errorf("list reviews for %s: %w", skillID, err)
	}
	return reviews, nil
}

// Unpublish removes a skill from the marketplace.
func (m *Marketplace) Unpublish(ctx context.Context, skillID string) error {
	if err := m.bridge.DeleteJSON(ctx, "/api/marketplace/"+skillID); err != nil {
		return fmt.Errorf("unpublish skill %s: %w", skillID, err)
	}
	return nil
}
