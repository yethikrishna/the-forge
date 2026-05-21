package cmd

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/forge/sword/internal/refactor"
	"github.com/spf13/cobra"
)

var refactorCmd = &cobra.Command{
	Use:   "refactor",
	Short: "Dependency-aware refactoring with migration plans",
	Long:  "Analyze blast radius, generate step-by-step plans, and execute refactoring in dependency order. Keeps the codebase compiling at every step.",
}

var (
	refactorDir      string
	refactorType     string
	refactorTarget   string
	refactorDryRun   bool
	refactorPlanID   string
	refactorOutput   string
)

func init() {
	refactorCmd.AddCommand(refactorScanCmd)
	refactorCmd.AddCommand(refactorAnalyzeCmd)
	refactorCmd.AddCommand(refactorPlanCmd)
	refactorCmd.AddCommand(refactorExecCmd)
	refactorCmd.AddCommand(refactorListCmd)
	refactorCmd.AddCommand(refactorShowCmd)
	refactorCmd.AddCommand(refactorExportCmd)

	refactorScanCmd.Flags().StringVarP(&refactorDir, "dir", "d", ".", "project directory to scan")
	refactorAnalyzeCmd.Flags().StringVarP(&refactorTarget, "target", "t", "", "target symbol or package")
	refactorAnalyzeCmd.Flags().StringVarP(&refactorType, "type", "", "rename-symbol", "refactoring type")
	refactorPlanCmd.Flags().StringVarP(&refactorTarget, "target", "t", "", "target symbol or package")
	refactorPlanCmd.Flags().StringVarP(&refactorType, "type", "", "rename-symbol", "refactoring type")
	refactorPlanCmd.Flags().StringVarP(&refactorDir, "dir", "d", ".", "project directory")
	refactorPlanCmd.Flags().BoolVarP(&refactorDryRun, "dry-run", "n", false, "dry run (no actual changes)")
	refactorExecCmd.Flags().StringVarP(&refactorPlanID, "plan", "p", "", "plan ID to execute")
	refactorShowCmd.Flags().StringVarP(&refactorPlanID, "plan", "p", "", "plan ID to show")
	refactorExportCmd.Flags().StringVarP(&refactorPlanID, "plan", "p", "", "plan ID to export")
	refactorExportCmd.Flags().StringVarP(&refactorOutput, "output", "o", "plan.json", "output file path")
}

var refactorScanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan project dependencies",
	Long:  "Scan Go source files and build a dependency index for refactoring analysis.",
	RunE: func(cmd *cobra.Command, args []string) error {
		absDir, err := filepath.Abs(refactorDir)
		if err != nil {
			return err
		}

		engine := refactor.NewEngine(absDir)
		if err := engine.ScanDependencies(cmd.Context()); err != nil {
			return fmt.Errorf("scan failed: %w", err)
		}

		stats := engine.Stats()
		fmt.Fprintf(cmd.OutOrStdout(), "Scanned %v dependencies, %v indexed symbols\n", stats["dependencies"], stats["indexed_symbols"])
		return nil
	},
}

var refactorAnalyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Analyze impact of a refactoring operation",
	Long:  "Perform impact analysis showing the blast radius of a proposed refactoring.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if refactorTarget == "" {
			return fmt.Errorf("target is required (--target)")
		}

		absDir, _ := filepath.Abs(refactorDir)
		engine := refactor.NewEngine(absDir)
		engine.ScanDependencies(cmd.Context())

		refType := refactor.RefactorType(refactorType)
		analysis, err := engine.AnalyzeImpact(cmd.Context(), refactorTarget, refType)
		if err != nil {
			return fmt.Errorf("impact analysis failed: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Impact Analysis: %s (%s)\n", analysis.Target, analysis.Type)
		fmt.Fprintf(cmd.OutOrStdout(), "  Risk Level:      %s\n", analysis.RiskLevel)
		fmt.Fprintf(cmd.OutOrStdout(), "  Direct Files:    %d\n", len(analysis.DirectFiles))
		fmt.Fprintf(cmd.OutOrStdout(), "  Indirect Files:  %d\n", len(analysis.IndirectFiles))
		fmt.Fprintf(cmd.OutOrStdout(), "  Affected Pkgs:   %d\n", len(analysis.AffectedPkgs))
		fmt.Fprintf(cmd.OutOrStdout(), "  Breaking Change: %v\n", analysis.BreakingChange)
		fmt.Fprintf(cmd.OutOrStdout(), "  Estimated Steps: %d\n", analysis.EstimatedSteps)

		if len(analysis.DirectFiles) > 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "\nDirect file impacts:")
			for _, f := range analysis.DirectFiles {
				rel, _ := filepath.Rel(absDir, f)
				fmt.Fprintf(cmd.OutOrStdout(), "  • %s\n", rel)
			}
		}

		return nil
	},
}

var refactorPlanCmd = &cobra.Command{
	Use:   "plan",
	Short: "Create a refactoring plan",
	Long:  "Generate a step-by-step migration plan for a refactoring operation.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if refactorTarget == "" {
			return fmt.Errorf("target is required (--target)")
		}

		absDir, _ := filepath.Abs(refactorDir)
		engine := refactor.NewEngine(absDir)
		engine.ScanDependencies(cmd.Context())

		refType := refactor.RefactorType(refactorType)
		name := fmt.Sprintf("%s-%s", refType, filepath.Base(refactorTarget))

		opts := []refactor.PlanOption{}
		if refactorDryRun {
			opts = append(opts, refactor.WithDryRun(true))
		}

		plan, err := engine.CreatePlan(cmd.Context(), name, refType, refactorTarget, opts...)
		if err != nil {
			return fmt.Errorf("plan creation failed: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Plan: %s (%s)\n", plan.Name, plan.ID)
		fmt.Fprintf(cmd.OutOrStdout(), "  Status:    %s\n", plan.Status)
		fmt.Fprintf(cmd.OutOrStdout(), "  Steps:     %d\n", plan.TotalSteps)
		fmt.Fprintf(cmd.OutOrStdout(), "  Files:     %d\n", plan.TotalFiles)
		fmt.Fprintf(cmd.OutOrStdout(), "  Dry Run:   %v\n", plan.DryRun)
		fmt.Fprintln(cmd.OutOrStdout(), "\nRisk Summary:")
		for risk, count := range plan.RiskSummary {
			fmt.Fprintf(cmd.OutOrStdout(), "  %-10s %d\n", risk, count)
		}

		fmt.Fprintln(cmd.OutOrStdout(), "\nSteps:")
		for _, step := range plan.Steps {
			auto := "auto"
			if !step.AutoApply {
				auto = "manual"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  %d. [%s] %s (%s)\n", step.Order, auto, step.Description, step.Risk)
		}

		return nil
	},
}

var refactorExecCmd = &cobra.Command{
	Use:   "exec",
	Short: "Execute a refactoring plan",
	Long:  "Execute a refactoring plan step by step, maintaining buildability at each step.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if refactorPlanID == "" {
			return fmt.Errorf("plan ID is required (--plan)")
		}

		absDir, _ := filepath.Abs(refactorDir)
		engine := refactor.NewEngine(absDir)

		if err := engine.ExecutePlan(cmd.Context(), refactorPlanID); err != nil {
			return fmt.Errorf("execution failed: %w", err)
		}

		plan, _ := engine.GetPlan(refactorPlanID)
		if plan != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "Plan %s: %s\n", plan.ID, plan.Status)
			for _, step := range plan.Steps {
				status := "skipped"
				if step.Applied {
					status = "applied"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %d. [%s] %s\n", step.Order, status, step.Description)
				if step.Notes != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "     → %s\n", step.Notes)
				}
			}
		}

		return nil
	},
}

var refactorListCmd = &cobra.Command{
	Use:   "list",
	Short: "List refactoring plans",
	RunE: func(cmd *cobra.Command, args []string) error {
		engine := refactor.NewEngine(".")
		plans := engine.ListPlans()

		if len(plans) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No refactoring plans found.")
			return nil
		}

		fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-20s %-15s %-10s %s\n", "ID", "Name", "Type", "Status", "Steps")
		fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("-", 90))
		for _, p := range plans {
			fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-20s %-15s %-10s %d\n", p.ID, p.Name, p.Type, p.Status, p.TotalSteps)
		}

		return nil
	},
}

var refactorShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show plan details",
	RunE: func(cmd *cobra.Command, args []string) error {
		if refactorPlanID == "" {
			return fmt.Errorf("plan ID is required (--plan)")
		}

		engine := refactor.NewEngine(".")
		plan, ok := engine.GetPlan(refactorPlanID)
		if !ok {
			return fmt.Errorf("plan %s not found", refactorPlanID)
		}

		data, err := json.MarshalIndent(plan, "", "  ")
		if err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	},
}

var refactorExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export plan as JSON",
	RunE: func(cmd *cobra.Command, args []string) error {
		if refactorPlanID == "" {
			return fmt.Errorf("plan ID is required (--plan)")
		}

		engine := refactor.NewEngine(".")
		if err := engine.ExportPlan(refactorPlanID, refactorOutput); err != nil {
			return fmt.Errorf("export failed: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Plan exported to %s\n", refactorOutput)
		return nil
	},
}
