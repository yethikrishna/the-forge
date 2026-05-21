package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	abtestpkg "github.com/forge/sword/internal/eval2/abtest"
	"github.com/forge/sword/internal/pretty"
	"github.com/spf13/cobra"
)

func abTestCmd() *cobra.Command {
	var experimentDir string
	cmd := &cobra.Command{
		Use:   "abtest",
		Short: "A/B testing for agent configurations",
		Long: `Run A/B tests comparing different agent configurations.
Examples:
  forge abtest create "Model comparison" --variant "A:claude-sonnet-4" --variant "B:gpt-4.1"
  forge abtest start <id>
  forge abtest record <id> --variant A --score 0.9 --latency 500 --cost 0.05
  forge abtest analyze <id>
  forge abtest list`,
	}
	createCmd := &cobra.Command{
		Use: "create <name>", Short: "Create an A/B test experiment", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getABDir(experimentDir)
			store := abtestpkg.NewStore(dir)
			vf, _ := cmd.Flags().GetStringSlice("variant")
			ss, _ := cmd.Flags().GetInt("samples")
			if len(vf) < 2 {
				return fmt.Errorf("need >= 2 variants")
			}
			var vs []abtestpkg.Variant
			for _, v := range vf {
				p := splitVar(v)
				if len(p) != 2 {
					return fmt.Errorf("bad variant: %s", v)
				}
				vs = append(vs, abtestpkg.Variant{Name: p[0], Model: p[1]})
			}
			exp, err := store.Create(args[0], "", vs, ss)
			if err != nil {
				return err
			}
			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Created: %s (%s)", exp.Name, exp.ID)))
			return nil
		},
	}
	createCmd.Flags().StringSlice("variant", nil, "Variant Name:Model")
	createCmd.Flags().Int("samples", 30, "Samples per variant")
	startCmd := &cobra.Command{
		Use: "start <id>", Short: "Start an experiment", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store := abtestpkg.NewStore(getABDir(experimentDir))
			exp, err := store.Start(args[0])
			if err != nil {
				return err
			}
			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Started: %s", exp.Name)))
			return nil
		},
	}
	recordCmd := &cobra.Command{
		Use: "record <id>", Short: "Record a test result", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store := abtestpkg.NewStore(getABDir(experimentDir))
			v, _ := cmd.Flags().GetString("variant")
			sc, _ := cmd.Flags().GetFloat64("score")
			lt, _ := cmd.Flags().GetInt("latency")
			co, _ := cmd.Flags().GetFloat64("cost")
			su, _ := cmd.Flags().GetBool("success")
			if v == "" {
				return fmt.Errorf("--variant required")
			}
			r := abtestpkg.Result{Variant: v, Score: sc, LatencyMS: lt, CostUSD: co, Success: su}
			exp, err := store.RecordResult(args[0], r)
			if err != nil {
				return err
			}
			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Recorded %s (status: %s)", v, exp.Status)))
			return nil
		},
	}
	recordCmd.Flags().String("variant", "", "Variant name")
	recordCmd.Flags().Float64("score", 0, "Quality score 0-1")
	recordCmd.Flags().Int("latency", 0, "Latency ms")
	recordCmd.Flags().Float64("cost", 0, "Cost USD")
	recordCmd.Flags().Bool("success", true, "Success")
	analyzeCmd := &cobra.Command{
		Use: "analyze <id>", Short: "Analyze results", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store := abtestpkg.NewStore(getABDir(experimentDir))
			exp, err := store.Get(args[0])
			if err != nil {
				return err
			}
			if len(exp.Results) == 0 {
				fmt.Println(pretty.InfoLine("No results"))
				return nil
			}
			a := abtestpkg.Analyze(exp)
			j, _ := cmd.Flags().GetBool("json")
			if j {
				data, _ := json.MarshalIndent(a, "", "  ")
				fmt.Println(string(data))
				return nil
			}
			fmt.Print(abtestpkg.FormatAnalysis(a))
			return nil
		},
	}
	analyzeCmd.Flags().Bool("json", false, "JSON output")
	listCmd := &cobra.Command{
		Use: "list", Short: "List experiments",
		RunE: func(cmd *cobra.Command, args []string) error {
			store := abtestpkg.NewStore(getABDir(experimentDir))
			exps, err := store.List()
			if err != nil {
				return err
			}
			if len(exps) == 0 {
				fmt.Println(pretty.InfoLine("No experiments"))
				return nil
			}
			fmt.Println(pretty.HeaderLine("A/B Test Experiments"))
			for _, e := range exps {
				fmt.Printf("  %-20s %-12s %s (%d results)\n", e.ID, e.Status, e.Name, len(e.Results))
			}
			return nil
		},
	}
	cmd.AddCommand(createCmd, startCmd, recordCmd, analyzeCmd, listCmd)
	cmd.PersistentFlags().StringVar(&experimentDir, "dir", "", "Data dir (default: .forge/abtest)")
	return cmd
}
func getABDir(f string) string {
	if f != "" {
		return f
	}
	wd, _ := os.Getwd()
	return filepath.Join(wd, ".forge", "abtest")
}
func splitVar(s string) []string {
	for i, c := range s {
		if c == ':' {
			return []string{s[:i], s[i+1:]}
		}
	}
	return nil
}
