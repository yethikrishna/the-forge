package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/forge/sword/internal/optimize/breed"
	"github.com/spf13/cobra"
)

func breedCmd() *cobra.Command {
	var generations int
	var popSize int
	var mutationRate float64
	var storeDir string

	cmd := &cobra.Command{
		Use:   "breed",
		Short: "Evolve agent configurations through genetic optimization",
		Long: `Breed better agents through natural selection.

The forge tests many blades. The strongest survive to sire the next generation.
Each run evaluates agent configurations (model, temperature, jail mode, etc.)
and evolves them toward higher fitness scores.

Fitness is measured by task success rate, response quality, and cost efficiency.

Examples:
  forge breed init                    # Initialize a new population
  forge breed evolve                  # Evolve one generation
  forge breed evolve --generations 10 # Evolve 10 generations
  forge breed best                    # Show the best genome
  forge breed stats                   # Show population statistics`,
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:   "init",
			Short: "Initialize a new breeding population",
			RunE: func(cmd *cobra.Command, args []string) error {
				store := storePath(storeDir)
				traits := breed.DefaultAgentTraits()

				fitness := simulatedFitness()
				e := breed.NewEvolver(traits, fitness, store)
				e.PopulationSize = popSize

				pop := e.Initialize()
				fmt.Println(breed.FormatGeneration(pop, 0))
				fmt.Printf("\nInitialized population of %d genomes.\n", len(pop))
				return nil
			},
		},
		&cobra.Command{
			Use:   "evolve",
			Short: "Evolve the population",
			RunE: func(cmd *cobra.Command, args []string) error {
				store := storePath(storeDir)
				traits := breed.DefaultAgentTraits()

				fitness := simulatedFitness()
				e := breed.NewEvolver(traits, fitness, store)
				e.PopulationSize = popSize
				e.MutationRate = mutationRate

				// Initialize if empty
				if len(e.Population()) == 0 {
					e.Initialize()
				}

				for i := 0; i < generations; i++ {
					pop := e.Evolve()
					fmt.Printf("Generation %d: best=%.4f avg=%.4f diversity=%.2f\n",
						i+1, e.BestFitness(), e.AverageFitness(), e.Diversity())

					if generations <= 5 || i%5 == 0 {
						fmt.Println(breed.FormatGeneration(pop, i+1))
					}
				}

				best := e.Best()
				if best != nil {
					fmt.Println("\n=== Best Genome ===")
					fmt.Printf("ID: %s\nFitness: %.4f\nGeneration: %d\n",
						best.ID, best.Fitness, best.Generation)
					for k, v := range best.Traits {
						fmt.Printf("  %-15s %s\n", k, v)
					}
				}

				return nil
			},
		},
		&cobra.Command{
			Use:   "best",
			Short: "Show the best genome",
			RunE: func(cmd *cobra.Command, args []string) error {
				store := storePath(storeDir)
				traits := breed.DefaultAgentTraits()

				e := breed.NewEvolver(traits, simulatedFitness(), store)

				best := e.Best()
				if best == nil {
					fmt.Println("No population found. Run 'forge breed init' first.")
					return nil
				}

				fmt.Printf("Best Genome: %s\n", best.ID)
				fmt.Printf("Fitness:     %.4f\n", best.Fitness)
				fmt.Printf("Runs:        %d\n", best.Runs)
				fmt.Printf("Generation:  %d\n", best.Generation)
				if best.Parent1 != "" {
					fmt.Printf("Parents:     %s x %s\n", best.Parent1, best.Parent2)
				}
				fmt.Println("\nTraits:")
				for k, v := range best.Traits {
					fmt.Printf("  %-15s %s\n", k, v)
				}

				return nil
			},
		},
		&cobra.Command{
			Use:   "stats",
			Short: "Show population statistics",
			RunE: func(cmd *cobra.Command, args []string) error {
				store := storePath(storeDir)
				traits := breed.DefaultAgentTraits()

				e := breed.NewEvolver(traits, simulatedFitness(), store)

				pop := e.Population()
				if len(pop) == 0 {
					fmt.Println("No population found. Run 'forge breed init' first.")
					return nil
				}

				fmt.Printf("Population:    %d\n", len(pop))
				fmt.Printf("Best Fitness:  %.4f\n", e.BestFitness())
				fmt.Printf("Avg Fitness:   %.4f\n", e.AverageFitness())
				fmt.Printf("Diversity:     %.2f\n", e.Diversity())

				// Trait distribution
				fmt.Println("\nTrait Distribution:")
				for _, t := range traits {
					counts := make(map[string]int)
					for _, g := range pop {
						counts[g.Traits[t.Name]]++
					}
					fmt.Printf("  %s:\n", t.Name)
					for v, c := range counts {
						fmt.Printf("    %-20s %d (%.0f%%)\n", v, c, float64(c)/float64(len(pop))*100)
					}
				}

				return nil
			},
		},
	)

	cmd.PersistentFlags().IntVar(&generations, "generations", 1, "Number of generations to evolve")
	cmd.PersistentFlags().IntVar(&popSize, "population", 20, "Population size")
	cmd.PersistentFlags().Float64Var(&mutationRate, "mutation-rate", 0.15, "Mutation rate (0-1)")
	cmd.PersistentFlags().StringVar(&storeDir, "store", "", "Storage directory (default: .forge/breed/)")

	return cmd
}

func storePath(dir string) string {
	if dir != "" {
		return filepath.Join(dir, "breed.json")
	}
	return filepath.Join(".forge", "breed", "breed.json")
}

// simulatedFitness provides a deterministic fitness function for demo/testing.
// In production, this would be replaced with actual agent evaluation.
func simulatedFitness() breed.FitnessFunc {
	return func(g breed.Genome) float64 {
		fitness := 0.5

		// Prefer claude-sonnet
		switch g.Traits["model"] {
		case "anthropic/claude-sonnet-4-20250514":
			fitness += 0.3
		case "anthropic/claude-opus-4-20250514":
			fitness += 0.2
		case "openai/gpt-4o":
			fitness += 0.15
		case "google/gemini-2.5-pro":
			fitness += 0.1
		}

		// Prefer moderate temperature
		temp := 0.7
		if v, ok := g.Traits["temperature"]; ok {
			fmt.Sscanf(v, "%f", &temp)
			deviation := (temp - 0.5) * (temp - 0.5)
			fitness -= deviation * 0.1
		}

		// Prefer strict jail
		if g.Traits["jail"] == "strict" {
			fitness += 0.05
		}

		// Bound
		if fitness > 1.0 {
			fitness = 1.0
		}
		if fitness < 0 {
			fitness = 0
		}

		return fitness
	}
}

// silence unused import
var _ = os.ReadFile
