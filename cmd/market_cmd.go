package cmd

import (
	"fmt"

	"github.com/forge/sword/internal/skillmarket"
	"github.com/spf13/cobra"
)

var marketCmd = &cobra.Command{
	Use:   "market",
	Short: "Skill marketplace",
	Long:  "Browse, publish, and manage reusable agent skills in the marketplace.",
}

var (
	marketDir    string
	marketCat    string
	marketAuthor string
	marketEntry  string
)

func init() {
	marketCmd.AddCommand(marketPublishCmd)
	marketCmd.AddCommand(marketSearchCmd)
	marketCmd.AddCommand(marketShowCmd)
	marketCmd.AddCommand(marketRateCmd)
	marketCmd.AddCommand(marketInstallCmd)
	marketCmd.AddCommand(marketTrendingCmd)
	marketCmd.AddCommand(marketTopRatedCmd)
	marketCmd.AddCommand(marketListCmd)
	marketCmd.AddCommand(marketDeprecateCmd)
	marketCmd.AddCommand(marketStatsCmd)

	marketCmd.PersistentFlags().StringVar(&marketDir, "dir", ".forge/market", "Marketplace storage directory")
	marketPublishCmd.Flags().StringVar(&marketCat, "category", "custom", "Skill category")
	marketPublishCmd.Flags().StringVar(&marketAuthor, "author", "", "Author name")
	marketPublishCmd.Flags().StringVar(&marketEntry, "entrypoint", "", "Skill entrypoint function")
	marketRateCmd.Flags().Int("score", 0, "Rating score (1-5)")
	marketRateCmd.Flags().String("review", "", "Review text")
}

func getMarket() (*skillmarket.Market, error) {
	return skillmarket.NewMarket(marketDir)
}

var marketPublishCmd = &cobra.Command{
	Use:   "publish [name] [description]",
	Short: "Publish a skill to the marketplace",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if marketAuthor == "" {
			return fmt.Errorf("--author is required")
		}
		market, err := getMarket()
		if err != nil {
			return err
		}
		skill := &skillmarket.Skill{
			Name:        args[0],
			Author:      marketAuthor,
			Category:    skillmarket.Category(marketCat),
			Description: args[1],
			Entrypoint:  marketEntry,
		}
		if err := market.Publish(skill); err != nil {
			return err
		}
		fmt.Printf("Published: %s (id: %s)\n", skill.Name, skill.ID)
		return nil
	},
}

var marketSearchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search skills",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		market, err := getMarket()
		if err != nil {
			return err
		}
		results := market.Search(args[0])
		if len(results) == 0 {
			fmt.Println("No matching skills.")
			return nil
		}
		fmt.Printf("Found %d skills:\n", len(results))
		for _, s := range results {
			fmt.Printf("  %s [%s] by %s — %s (%.1f★, %d downloads)\n",
				s.Name, s.Category, s.Author, s.Description, s.Rating, s.Downloads)
		}
		return nil
	},
}

var marketShowCmd = &cobra.Command{
	Use:   "show [id-or-name]",
	Short: "Show skill details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		market, err := getMarket()
		if err != nil {
			return err
		}
		s, ok := market.Get(args[0])
		if !ok {
			s, ok = market.GetByName(args[0])
		}
		if !ok {
			return fmt.Errorf("skill %q not found", args[0])
		}
		fmt.Printf("Skill: %s (v%s)\n", s.Name, s.Version)
		fmt.Printf("Author: %s  Category: %s\n", s.Author, s.Category)
		fmt.Printf("Description: %s\n", s.Description)
		fmt.Printf("Rating: %.1f/5 (%d ratings)  Downloads: %d\n", s.Rating, s.RatingCount, s.Downloads)
		fmt.Printf("Status: %s  Entrypoint: %s\n", s.Status, s.Entrypoint)
		if len(s.Tags) > 0 {
			fmt.Printf("Tags: %v\n", s.Tags)
		}
		return nil
	},
}

var marketRateCmd = &cobra.Command{
	Use:   "rate [skill-id] [user-id]",
	Short: "Rate a skill",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		score, _ := cmd.Flags().GetInt("score")
		review, _ := cmd.Flags().GetString("review")
		if score < 1 || score > 5 {
			return fmt.Errorf("--score must be 1-5")
		}
		market, err := getMarket()
		if err != nil {
			return err
		}
		return market.Rate(args[0], args[1], score, review)
	},
}

var marketInstallCmd = &cobra.Command{
	Use:   "install [skill-id]",
	Short: "Install a skill (increments download)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		market, err := getMarket()
		if err != nil {
			return err
		}
		if err := market.Download(args[0]); err != nil {
			return err
		}
		fmt.Println("Skill installed.")
		return nil
	},
}

var marketTrendingCmd = &cobra.Command{
	Use:   "trending",
	Short: "Show trending skills",
	RunE: func(cmd *cobra.Command, args []string) error {
		market, err := getMarket()
		if err != nil {
			return err
		}
		skills := market.Trending(10)
		if len(skills) == 0 {
			fmt.Println("No skills available.")
			return nil
		}
		fmt.Println("Trending:")
		for i, s := range skills {
			fmt.Printf("  %d. %s (%d downloads, %.1f★)\n", i+1, s.Name, s.Downloads, s.Rating)
		}
		return nil
	},
}

var marketTopRatedCmd = &cobra.Command{
	Use:   "top-rated",
	Short: "Show highest rated skills",
	RunE: func(cmd *cobra.Command, args []string) error {
		market, err := getMarket()
		if err != nil {
			return err
		}
		skills := market.TopRated(10)
		if len(skills) == 0 {
			fmt.Println("No rated skills.")
			return nil
		}
		fmt.Println("Top Rated:")
		for i, s := range skills {
			fmt.Printf("  %d. %s (%.1f★, %d ratings)\n", i+1, s.Name, s.Rating, s.RatingCount)
		}
		return nil
	},
}

var marketListCmd = &cobra.Command{
	Use:   "list",
	Short: "List skills by category",
	RunE: func(cmd *cobra.Command, args []string) error {
		market, err := getMarket()
		if err != nil {
			return err
		}
		cat := skillmarket.Category(marketCat)
		skills := market.ListByCategory(cat)
		if len(skills) == 0 {
			fmt.Println("No skills in this category.")
			return nil
		}
		fmt.Printf("Skills (%s, %d):\n", cat, len(skills))
		for _, s := range skills {
			fmt.Printf("  %s by %s (%d downloads)\n", s.Name, s.Author, s.Downloads)
		}
		return nil
	},
}

var marketDeprecateCmd = &cobra.Command{
	Use:   "deprecate [skill-id]",
	Short: "Deprecate a skill",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		market, err := getMarket()
		if err != nil {
			return err
		}
		return market.Deprecate(args[0])
	},
}

var marketStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Marketplace statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		market, err := getMarket()
		if err != nil {
			return err
		}
		stats := market.Stats()
		fmt.Printf("Skills: %d (published: %d, deprecated: %d)\n", stats.TotalSkills, stats.Published, stats.Deprecated)
		fmt.Printf("Downloads: %d  Avg rating: %.1f\n", stats.TotalDownloads, stats.AvgRating)
		for cat, count := range stats.ByCategory {
			fmt.Printf("  %s: %d\n", cat, count)
		}
		return nil
	},
}
