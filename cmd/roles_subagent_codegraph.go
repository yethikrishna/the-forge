package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/forge/sword/internal/agentrole"
	"github.com/forge/sword/internal/codegraph"
	"github.com/forge/sword/internal/subagent"
	"github.com/spf13/cobra"
)

func subagentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "subagent",
		Short: "Spawn and manage sub-agents for parallel task execution",
		Long:  `Spawn sub-agents that execute tasks in parallel with dependency tracking, cost budgets, and lifecycle hooks.`,
	}

	var outputJSON bool
	cmd.PersistentFlags().BoolVar(&outputJSON, "json", false, "Output as JSON")

	// spawn
	spawnCmd := &cobra.Command{
		Use:   "spawn <name> <prompt>",
		Short: "Spawn a new sub-agent task",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			spawner := subagent.NewSpawner(subagent.DefaultSpawnConfig())

			var opts []subagent.SpawnOption
			if model, _ := cmd.Flags().GetString("model"); model != "" {
				opts = append(opts, subagent.WithModel(model))
			}
			if role, _ := cmd.Flags().GetString("role"); role != "" {
				opts = append(opts, subagent.WithRole(role))
			}
			if timeout, _ := cmd.Flags().GetDuration("timeout"); timeout > 0 {
				opts = append(opts, subagent.WithTimeout(timeout))
			}

			task, err := spawner.Spawn(cmd.Context(), args[0], args[1], opts...)
			if err != nil {
				return err
			}

			result := spawner.Execute(cmd.Context(), task)

			if outputJSON {
				data, _ := json.MarshalIndent(map[string]interface{}{
					"task":   task,
					"result": result,
				}, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Printf("Task: %s\n", task.ID)
			fmt.Printf("State: %s\n", task.State)
			fmt.Printf("Duration: %v\n", result.Duration)
			fmt.Printf("Output:\n%s\n", result.Output)
			return nil
		},
	}
	spawnCmd.Flags().String("model", "", "Model to use")
	spawnCmd.Flags().String("role", "", "Agent role")
	spawnCmd.Flags().Duration("timeout", 5*time.Minute, "Task timeout")

	// list
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List sub-agent tasks",
		RunE: func(cmd *cobra.Command, args []string) error {
			spawner := subagent.NewSpawner(subagent.DefaultSpawnConfig())

			state, _ := cmd.Flags().GetString("state")
			tasks := spawner.ListTasks(subagent.TaskState(state))

			if outputJSON {
				data, _ := json.MarshalIndent(tasks, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			if len(tasks) == 0 {
				fmt.Println("No tasks found.")
				return nil
			}

			fmt.Printf("%-12s %-20s %-10s %s\n", "ID", "NAME", "STATE", "DURATION")
			for _, t := range tasks {
				fmt.Println(subagent.FormatTask(t))
			}
			return nil
		},
	}
	listCmd.Flags().String("state", "", "Filter by state")

	// stats
	statsCmd := &cobra.Command{
		Use:   "stats",
		Short: "Show sub-agent spawner statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			spawner := subagent.NewSpawner(subagent.DefaultSpawnConfig())
			stats := spawner.Stats()

			if outputJSON {
				data, _ := json.MarshalIndent(stats, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Println(subagent.FormatStats(stats))
			return nil
		},
	}

	cmd.AddCommand(spawnCmd, listCmd, statsCmd)
	return cmd
}

func agentRoleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "role",
		Short: "Manage agent roles for orchestration",
		Long:  `Define and assign roles (planner, coder, tester, reviewer, etc.) to agents for structured multi-agent orchestration.`,
	}

	var outputJSON bool
	var storeDir string
	cmd.PersistentFlags().BoolVar(&outputJSON, "json", false, "Output as JSON")
	cmd.PersistentFlags().StringVar(&storeDir, "dir", ".forge/roles", "Role storage directory")

	// list
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List available roles",
		RunE: func(cmd *cobra.Command, args []string) error {
			reg := agentrole.NewRegistry(storeDir)

			category, _ := cmd.Flags().GetString("category")
			roles := reg.List(agentrole.RoleCategory(category))

			if outputJSON {
				data, _ := json.MarshalIndent(roles, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			if len(roles) == 0 {
				fmt.Println("No roles found.")
				return nil
			}

			fmt.Printf("%-15s %-12s %-30s\n", "NAME", "CATEGORY", "DESCRIPTION")
			for _, r := range roles {
				fmt.Println(agentrole.FormatRole(r))
			}
			return nil
		},
	}
	listCmd.Flags().String("category", "", "Filter by category")

	// show
	showCmd := &cobra.Command{
		Use:   "show <role-name>",
		Short: "Show role details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			reg := agentrole.NewRegistry(storeDir)
			role, err := reg.Get(args[0])
			if err != nil {
				return err
			}

			if outputJSON {
				data, _ := json.MarshalIndent(role, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Printf("Role: %s (%s)\n", role.DisplayName, role.Name)
			fmt.Printf("  Category:    %s\n", role.Category)
			fmt.Printf("  Description: %s\n", role.Description)
			fmt.Printf("  Model Hint:  %s\n", role.ModelHint)
			fmt.Printf("  Priority:    %d\n", role.Priority)
			fmt.Printf("  Tools:       %v\n", role.Tools)
			fmt.Printf("  Traits:      %v\n", role.Traits)
			fmt.Printf("  Constraints: %v\n", role.Constraints)
			if role.SystemPrompt != "" {
				fmt.Printf("  System Prompt:\n    %s\n", role.SystemPrompt)
			}
			return nil
		},
	}

	// categories
	catCmd := &cobra.Command{
		Use:   "categories",
		Short: "List role categories",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(agentrole.FormatCategories())
			return nil
		},
	}

	// assign
	assignCmd := &cobra.Command{
		Use:   "assign <agent-id> <role-name>",
		Short: "Assign a role to an agent",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			reg := agentrole.NewRegistry(storeDir)
			session, _ := cmd.Flags().GetString("session")

			asgn, err := reg.Assign(args[0], args[1], session)
			if err != nil {
				return err
			}

			if outputJSON {
				data, _ := json.MarshalIndent(asgn, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Printf("Assigned role %s to agent %s (assignment: %s)\n", args[1], args[0], asgn.ID)
			return nil
		},
	}
	assignCmd.Flags().String("session", "", "Session ID")

	// revoke
	revokeCmd := &cobra.Command{
		Use:   "revoke <assignment-id>",
		Short: "Revoke a role assignment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			reg := agentrole.NewRegistry(storeDir)
			if err := reg.Revoke(args[0]); err != nil {
				return err
			}
			fmt.Printf("Revoked assignment %s\n", args[0])
			return nil
		},
	}

	// validate
	validateCmd := &cobra.Command{
		Use:   "validate <agent-id> <tool>",
		Short: "Check if an agent can use a tool",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			reg := agentrole.NewRegistry(storeDir)
			allowed := reg.Validate(args[0], args[1])
			if outputJSON {
				data, _ := json.Marshal(map[string]bool{"allowed": allowed})
				fmt.Println(string(data))
				return nil
			}
			if allowed {
				fmt.Printf("Agent %s CAN use tool %s\n", args[0], args[1])
			} else {
				fmt.Printf("Agent %s CANNOT use tool %s\n", args[0], args[1])
			}
			return nil
		},
	}

	// stats
	roleStatsCmd := &cobra.Command{
		Use:   "stats",
		Short: "Show role registry statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			reg := agentrole.NewRegistry(storeDir)
			stats := reg.Stats()

			if outputJSON {
				data, _ := json.MarshalIndent(stats, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Printf("Role Registry Stats:\n")
			fmt.Printf("  Total Roles: %d\n", stats.TotalRoles)
			fmt.Printf("  Active Assignments: %d\n", stats.ActiveAssignments)
			fmt.Printf("  By Category:\n")
			for cat, count := range stats.ByCategory {
				fmt.Printf("    %-12s %d\n", cat, count)
			}
			return nil
		},
	}

	cmd.AddCommand(listCmd, showCmd, catCmd, assignCmd, revokeCmd, validateCmd, roleStatsCmd)
	return cmd
}

func codegraphCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "codegraph",
		Short: "Code knowledge graph for dependency and impact analysis",
		Long:  `Build and query a code knowledge graph. Analyze dependencies, trace call chains, and assess change impact.`,
	}

	var outputJSON bool
	var storeDir string
	cmd.PersistentFlags().BoolVar(&outputJSON, "json", false, "Output as JSON")
	cmd.PersistentFlags().StringVar(&storeDir, "dir", ".forge/codegraph", "Graph storage directory")

	// stats
	statsCmd := &cobra.Command{
		Use:   "stats",
		Short: "Show graph statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			g := codegraph.NewGraph(storeDir)
			stats := g.Stats()

			if outputJSON {
				data, _ := json.MarshalIndent(stats, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Println(codegraph.FormatStats(stats))
			return nil
		},
	}

	// find
	findCmd := &cobra.Command{
		Use:   "find <pattern>",
		Short: "Find nodes by name pattern",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			g := codegraph.NewGraph(storeDir)
			nodeType, _ := cmd.Flags().GetString("type")
			nodes := g.FindNodes(args[0], codegraph.NodeType(nodeType))

			if outputJSON {
				data, _ := json.MarshalIndent(nodes, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			if len(nodes) == 0 {
				fmt.Println("No nodes found.")
				return nil
			}

			for _, n := range nodes {
				fmt.Printf("%-15s %-10s %-30s %s:%d\n", n.Name, n.Type, n.Package, n.File, n.Line)
			}
			return nil
		},
	}
	findCmd.Flags().String("type", "", "Filter by node type")

	// impact
	impactCmd := &cobra.Command{
		Use:   "impact <node-id>",
		Short: "Analyze change impact for a node",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			g := codegraph.NewGraph(storeDir)
			maxDepth, _ := cmd.Flags().GetInt("depth")
			if maxDepth <= 0 {
				maxDepth = 5
			}

			report := g.ImpactAnalysis(args[0], maxDepth)

			if outputJSON {
				data, _ := json.MarshalIndent(report, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Println(codegraph.FormatImpactReport(report))
			return nil
		},
	}
	impactCmd.Flags().Int("depth", 5, "Max traversal depth")

	// chain
	chainCmd := &cobra.Command{
		Use:   "chain <source-id> <target-id>",
		Short: "Trace call chain between two nodes",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			g := codegraph.NewGraph(storeDir)
			chain := g.CallChain(args[0], args[1])

			if outputJSON {
				data, _ := json.MarshalIndent(chain, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Println(codegraph.FormatCallChain(chain))
			return nil
		},
	}

	// neighbors
	neighborsCmd := &cobra.Command{
		Use:   "neighbors <node-id>",
		Short: "Show neighboring nodes",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			g := codegraph.NewGraph(storeDir)
			neighbors := g.Neighbors(args[0])

			if outputJSON {
				data, _ := json.MarshalIndent(neighbors, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			for _, n := range neighbors {
				fmt.Printf("%-15s %-10s %s\n", n.Name, n.Type, n.Package)
			}
			return nil
		},
	}

	// index (add nodes from codebase)
	indexCmd := &cobra.Command{
		Use:   "index <path>",
		Short: "Index a Go package into the graph",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			g := codegraph.NewGraph(storeDir)

			// Parse Go source files and build the graph
			count := indexGoPackage(g, args[0])

			if err := g.Save(); err != nil {
				return err
			}

			if outputJSON {
				data, _ := json.Marshal(map[string]interface{}{"indexed": count})
				fmt.Println(string(data))
				return nil
			}

			fmt.Printf("Indexed %d nodes from %s\n", count, args[0])
			return nil
		},
	}

	cmd.AddCommand(statsCmd, findCmd, impactCmd, chainCmd, neighborsCmd, indexCmd)
	return cmd
}

// indexGoPackage parses a Go package directory and adds nodes to the graph.
// This is a simplified parser — full production would use go/packages.
func indexGoPackage(g *codegraph.Graph, path string) int {
	count := 0

	// Add package node
	pkgName := filepath.Base(path)
	pkgID := g.AddNode(codegraph.Node{
		Type:    codegraph.NodePackage,
		Name:    pkgName,
		Package: pkgName,
	})
	count++

	// Walk Go files
	entries, err := os.ReadDir(path)
	if err != nil {
		return count
	}

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}

		fileID := g.AddNode(codegraph.Node{
			Type:    codegraph.NodeFile,
			Name:    entry.Name(),
			Package: pkgName,
			File:    filepath.Join(path, entry.Name()),
		})
		count++

		g.AddEdge(codegraph.Edge{
			Source: pkgID,
			Target: fileID,
			Type:   codegraph.EdgeContains,
		})
	}

	return count
}
