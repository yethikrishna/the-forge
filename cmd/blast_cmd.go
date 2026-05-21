package cmd

import (
	"fmt"

	"github.com/forge/sword/internal/blast"
	"github.com/spf13/cobra"
)

var blastCmd = &cobra.Command{
	Use:   "blast",
	Short: "Dependency blast radius analysis",
	Long:  `Analyze the blast radius of code changes. Know what's affected before you deploy.`,
}

func init() {
	blastCmd.AddCommand(blastAnalyzeCmd)
	blastCmd.AddCommand(blastDependentsCmd)
	blastCmd.AddCommand(blastDepsCmd)
}

// blast analyze
var blastAnalyzeCmd = &cobra.Command{
	Use:   "analyze [files...]",
	Short: "Analyze blast radius of changes",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := cmd.Flags().GetString("dir")
		if dir == "" {
			dir = "."
		}

		a := blast.NewAnalyzer(dir)

		var changes []blast.Change
		for _, f := range args {
			pkg := fileToPackage(f)
			changes = append(changes, blast.Change{
				Path:    f,
				Package: pkg,
			})
		}

		result := a.Analyze(changes)
		fmt.Println(blast.RenderResult(result))
		return nil
	},
}

// blast dependents
var blastDependentsCmd = &cobra.Command{
	Use:   "dependents [package]",
	Short: "Show packages that depend on a package",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := cmd.Flags().GetString("dir")
		if dir == "" {
			dir = "."
		}
		depth, _ := cmd.Flags().GetInt("depth")

		a := blast.NewAnalyzer(dir)
		dependents := a.PackageDependents(args[0], depth)

		if len(dependents) == 0 {
			fmt.Println("No dependents found")
		} else {
			for _, dep := range dependents {
				fmt.Printf("  [%s] %s (depth %d) — %s\n", dep.Level, dep.Package, dep.Depth, dep.Reason)
			}
		}
		return nil
	},
}

// blast deps
var blastDepsCmd = &cobra.Command{
	Use:   "deps [package]",
	Short: "Show dependencies of a package",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := cmd.Flags().GetString("dir")
		if dir == "" {
			dir = "."
		}

		a := blast.NewAnalyzer(dir)
		deps := a.PackageDependencies(args[0])

		if len(deps) == 0 {
			fmt.Println("No internal dependencies")
		} else {
			for _, dep := range deps {
				fmt.Printf("  %s\n", dep)
			}
		}
		return nil
	},
}

func fileToPackage(f string) string {
	lastSlash := -1
	for i := len(f) - 1; i >= 0; i-- {
		if f[i] == '/' {
			lastSlash = i
			break
		}
	}
	if lastSlash >= 0 {
		return f[:lastSlash]
	}
	return "."
}

func init() {
	blastAnalyzeCmd.Flags().String("dir", ".", "Project root directory")
	blastDependentsCmd.Flags().String("dir", ".", "Project root directory")
	blastDependentsCmd.Flags().Int("depth", 3, "Max dependency depth")
	blastDepsCmd.Flags().String("dir", ".", "Project root directory")
}
