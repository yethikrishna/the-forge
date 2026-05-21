package skillmarket

import (
	"testing"
)

func TestPublishSkill(t *testing.T) {
	dir := t.TempDir()
	market, err := NewMarket(dir)
	if err != nil {
		t.Fatalf("NewMarket: %v", err)
	}

	skill := &Skill{
		Name:        "code-review",
		Author:      "forge-team",
		Category:    CatReview,
		Description: "Automated code review with security focus",
		Entrypoint:  "review_code",
		Tags:        []string{"review", "security", "quality"},
	}

	if err := market.Publish(skill); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if skill.ID == "" {
		t.Error("Expected skill ID")
	}
	if skill.Status != StatusPublished {
		t.Errorf("Expected published, got %s", skill.Status)
	}
}

func TestGetSkill(t *testing.T) {
	dir := t.TempDir()
	market, _ := NewMarket(dir)

	skill := &Skill{Name: "test", Author: "author", Category: CatCoding, Entrypoint: "test"}
	market.Publish(skill)

	retrieved, ok := market.Get(skill.ID)
	if !ok {
		t.Fatal("Expected to find skill")
	}
	if retrieved.Name != "test" {
		t.Errorf("Expected 'test', got %q", retrieved.Name)
	}
}

func TestGetByName(t *testing.T) {
	dir := t.TempDir()
	market, _ := NewMarket(dir)

	skill := &Skill{Name: "unique-skill", Author: "author", Category: CatCoding, Entrypoint: "run"}
	market.Publish(skill)

	retrieved, ok := market.GetByName("unique-skill")
	if !ok {
		t.Fatal("Expected to find skill by name")
	}
	if retrieved.Name != "unique-skill" {
		t.Errorf("Expected 'unique-skill', got %q", retrieved.Name)
	}
}

func TestSearch(t *testing.T) {
	dir := t.TempDir()
	market, _ := NewMarket(dir)

	market.Publish(&Skill{Name: "code-review", Author: "a", Category: CatReview, Description: "Review code", Entrypoint: "review", Tags: []string{"security"}})
	market.Publish(&Skill{Name: "bug-finder", Author: "b", Category: CatCoding, Description: "Find bugs", Entrypoint: "find", Tags: []string{"debugging"}})

	results := market.Search("review")
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}
}

func TestListByCategory(t *testing.T) {
	dir := t.TempDir()
	market, _ := NewMarket(dir)

	market.Publish(&Skill{Name: "skill-1", Author: "a", Category: CatReview, Entrypoint: "r"})
	market.Publish(&Skill{Name: "skill-2", Author: "a", Category: CatCoding, Entrypoint: "c"})
	market.Publish(&Skill{Name: "skill-3", Author: "a", Category: CatReview, Entrypoint: "r2"})

	reviewSkills := market.ListByCategory(CatReview)
	if len(reviewSkills) != 2 {
		t.Errorf("Expected 2 review skills, got %d", len(reviewSkills))
	}
}

func TestRateSkill(t *testing.T) {
	dir := t.TempDir()
	market, _ := NewMarket(dir)

	skill := &Skill{Name: "test", Author: "a", Category: CatCoding, Entrypoint: "run"}
	market.Publish(skill)

	market.Rate(skill.ID, "user-1", 5, "Excellent!")
	market.Rate(skill.ID, "user-2", 3, "Okay")

	retrieved, _ := market.Get(skill.ID)
	if retrieved.RatingCount != 2 {
		t.Errorf("Expected 2 ratings, got %d", retrieved.RatingCount)
	}
	if retrieved.Rating != 4.0 {
		t.Errorf("Expected 4.0 average, got %.1f", retrieved.Rating)
	}
}

func TestRateInvalidScore(t *testing.T) {
	dir := t.TempDir()
	market, _ := NewMarket(dir)

	skill := &Skill{Name: "test", Author: "a", Category: CatCoding, Entrypoint: "run"}
	market.Publish(skill)

	err := market.Rate(skill.ID, "user-1", 6, "Too high")
	if err == nil {
		t.Error("Expected error for invalid score")
	}
}

func TestDownload(t *testing.T) {
	dir := t.TempDir()
	market, _ := NewMarket(dir)

	skill := &Skill{Name: "test", Author: "a", Category: CatCoding, Entrypoint: "run"}
	market.Publish(skill)

	market.Download(skill.ID)
	market.Download(skill.ID)

	retrieved, _ := market.Get(skill.ID)
	if retrieved.Downloads != 2 {
		t.Errorf("Expected 2 downloads, got %d", retrieved.Downloads)
	}
}

func TestTrending(t *testing.T) {
	dir := t.TempDir()
	market, _ := NewMarket(dir)

	s1 := &Skill{Name: "popular", Author: "a", Category: CatCoding, Entrypoint: "run"}
	market.Publish(s1)
	for i := 0; i < 10; i++ {
		market.Download(s1.ID)
	}

	s2 := &Skill{Name: "less-popular", Author: "a", Category: CatCoding, Entrypoint: "run2"}
	market.Publish(s2)
	market.Download(s2.ID)

	trending := market.Trending(5)
	if len(trending) < 2 {
		t.Errorf("Expected 2 trending, got %d", len(trending))
	}
	if trending[0].Name != "popular" {
		t.Errorf("Expected 'popular' first, got %q", trending[0].Name)
	}
}

func TestTopRated(t *testing.T) {
	dir := t.TempDir()
	market, _ := NewMarket(dir)

	s1 := &Skill{Name: "best", Author: "a", Category: CatCoding, Entrypoint: "run"}
	market.Publish(s1)
	market.Rate(s1.ID, "u1", 5, "")

	s2 := &Skill{Name: "okay", Author: "a", Category: CatCoding, Entrypoint: "run2"}
	market.Publish(s2)
	market.Rate(s2.ID, "u1", 3, "")

	top := market.TopRated(5)
	if len(top) < 2 {
		t.Errorf("Expected 2 top rated, got %d", len(top))
	}
	if top[0].Name != "best" {
		t.Errorf("Expected 'best' first, got %q", top[0].Name)
	}
}

func TestDeprecate(t *testing.T) {
	dir := t.TempDir()
	market, _ := NewMarket(dir)

	skill := &Skill{Name: "test", Author: "a", Category: CatCoding, Entrypoint: "run"}
	market.Publish(skill)

	market.Deprecate(skill.ID)

	retrieved, _ := market.Get(skill.ID)
	if retrieved.Status != StatusDeprecated {
		t.Errorf("Expected deprecated, got %s", retrieved.Status)
	}
}

func TestRemove(t *testing.T) {
	dir := t.TempDir()
	market, _ := NewMarket(dir)

	skill := &Skill{Name: "test", Author: "a", Category: CatCoding, Entrypoint: "run"}
	market.Publish(skill)

	market.Remove(skill.ID)

	_, ok := market.Get(skill.ID)
	if ok {
		t.Error("Expected skill to be removed")
	}
}

func TestCategories(t *testing.T) {
	dir := t.TempDir()
	market, _ := NewMarket(dir)

	market.Publish(&Skill{Name: "a", Author: "a", Category: CatCoding, Entrypoint: "c"})
	market.Publish(&Skill{Name: "b", Author: "a", Category: CatReview, Entrypoint: "r"})

	cats := market.Categories()
	if len(cats) < 2 {
		t.Errorf("Expected 2 categories, got %d", len(cats))
	}
}

func TestStats(t *testing.T) {
	dir := t.TempDir()
	market, _ := NewMarket(dir)

	market.Publish(&Skill{Name: "a", Author: "a", Category: CatCoding, Entrypoint: "c"})
	market.Publish(&Skill{Name: "b", Author: "a", Category: CatReview, Entrypoint: "r"})

	stats := market.Stats()
	if stats.TotalSkills != 2 {
		t.Errorf("Expected 2 skills, got %d", stats.TotalSkills)
	}
	if stats.Published != 2 {
		t.Errorf("Expected 2 published, got %d", stats.Published)
	}
}
