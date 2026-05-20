package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/forge/sword/internal/memory"
	"github.com/forge/sword/internal/pretty"
	"github.com/spf13/cobra"
)

func memoryCmd() *cobra.Command {
	var memoryDir string

	cmd := &cobra.Command{
		Use:   "memory",
		Short: "Manage persistent agent memory",
		Long: `Store, search, and manage persistent agent memories.

Agents can store important context across sessions —
preferences, decisions, learned patterns — and retrieve
them later via semantic search.

Examples:
  forge memory store "User prefers dark mode" --tags ui,preference
  forge memory search "dark mode"
  forge memory list --agent claude
  forge memory show mem-123
  forge memory delete mem-123
  forge memory export --output memories.json`,
	}

	cmd.PersistentFlags().StringVar(&memoryDir, "dir", "~/.forge/memory", "Memory store directory")

	cmd.AddCommand(
		&cobra.Command{
			Use:   "store [content]",
			Short: "Store a new memory",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				store := getMemoryStore(memoryDir)
				content := args[0]

				tags, _ := cmd.Flags().GetStringSlice("tags")
				agent, _ := cmd.Flags().GetString("agent")
				session, _ := cmd.Flags().GetString("session")

				m := store.Store(agent, session, content, tags, nil)
				fmt.Println(pretty.SuccessLine(fmt.Sprintf("Memory stored: %s", m.ID)))
				fmt.Printf("  Agent:   %s\n", m.Agent)
				fmt.Printf("  Tags:    %s\n", strings.Join(m.Tags, ", "))
				fmt.Printf("  Content: %s\n", truncate(m.Content, 80))
				return nil
			},
		},
		&cobra.Command{
			Use:   "search [query]",
			Short: "Search memories",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				store := getMemoryStore(memoryDir)
				query := args[0]
				limit, _ := cmd.Flags().GetInt("limit")

				results := store.Search(query, limit)
				if len(results) == 0 {
					fmt.Println(pretty.InfoLine("No memories found matching query"))
					return nil
				}

				fmt.Println(pretty.HeaderLine(fmt.Sprintf("Search: %s (%d results)", query, len(results))))
				for _, m := range results {
					fmt.Printf("  %s  [%s] %s\n", m.ID, m.Agent, truncate(m.Content, 60))
					if len(m.Tags) > 0 {
						fmt.Printf("     Tags: %s\n", strings.Join(m.Tags, ", "))
					}
				}
				return nil
			},
		},
		&cobra.Command{
			Use:   "list",
			Short: "List memories",
			RunE: func(cmd *cobra.Command, args []string) error {
				store := getMemoryStore(memoryDir)
				agent, _ := cmd.Flags().GetString("agent")
				tag, _ := cmd.Flags().GetString("tag")
				limit, _ := cmd.Flags().GetInt("limit")

				var results []*memory.Memory
				if agent != "" {
					results = store.ListByAgent(agent)
				} else if tag != "" {
					results = store.ListByTag(tag)
				} else {
					results = store.ListRecent(limit)
				}

				if len(results) == 0 {
					fmt.Println(pretty.InfoLine("No memories stored yet"))
					return nil
				}

				fmt.Println(pretty.HeaderLine(fmt.Sprintf("Memories (%d)", len(results))))
				for _, m := range results {
					fmt.Printf("  %s  [%s] %s\n", m.ID, m.Agent, truncate(m.Content, 60))
				}
				return nil
			},
		},
		&cobra.Command{
			Use:   "show [id]",
			Short: "Show a specific memory",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				store := getMemoryStore(memoryDir)
				m, ok := store.Get(args[0])
				if !ok {
					return fmt.Errorf("memory %q not found", args[0])
				}

				fmt.Println(pretty.HeaderLine(fmt.Sprintf("Memory: %s", m.ID)))
				fmt.Printf("  Agent:     %s\n", m.Agent)
				fmt.Printf("  Session:   %s\n", m.Session)
				fmt.Printf("  Created:   %s\n", m.CreatedAt.Format("2006-01-02 15:04:05"))
				fmt.Printf("  Tags:      %s\n", strings.Join(m.Tags, ", "))
				fmt.Printf("  Content:   %s\n", m.Content)
				if len(m.Metadata) > 0 {
					fmt.Println("  Metadata:")
					for k, v := range m.Metadata {
						fmt.Printf("    %s: %s\n", k, v)
					}
				}
				return nil
			},
		},
		&cobra.Command{
			Use:   "delete [id]",
			Short: "Delete a memory",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				store := getMemoryStore(memoryDir)
				if !store.Delete(args[0]) {
					return fmt.Errorf("memory %q not found", args[0])
				}
				fmt.Println(pretty.SuccessLine(fmt.Sprintf("Memory %s deleted", args[0])))
				return nil
			},
		},
		&cobra.Command{
			Use:   "export",
			Short: "Export all memories as JSON",
			RunE: func(cmd *cobra.Command, args []string) error {
				store := getMemoryStore(memoryDir)
				output, _ := cmd.Flags().GetString("output")

				data, err := store.Export()
				if err != nil {
					return fmt.Errorf("export: %w", err)
				}

				if output != "" {
					return os.WriteFile(output, data, 0o644)
				}

				fmt.Println(string(data))
				return nil
			},
		},
		&cobra.Command{
			Use:   "import [file]",
			Short: "Import memories from JSON",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				store := getMemoryStore(memoryDir)

				data, err := os.ReadFile(args[0])
				if err != nil {
					return fmt.Errorf("read: %w", err)
				}

				if err := store.Import(data); err != nil {
					return fmt.Errorf("import: %w", err)
				}

				fmt.Println(pretty.SuccessLine(fmt.Sprintf("Imported %d memories", store.Count())))
				return nil
			},
		},
		&cobra.Command{
			Use:   "stats",
			Short: "Show memory store statistics",
			RunE: func(cmd *cobra.Command, args []string) error {
				store := getMemoryStore(memoryDir)

				fmt.Println(pretty.HeaderLine("Memory Statistics"))
				fmt.Printf("  Total memories: %d\n", store.Count())
				fmt.Printf("  Agents:         %v\n", store.Agents())
				fmt.Printf("  Tags:           %v\n", store.Tags())
				return nil
			},
		},
	)

	// Store flags
	cmd.Commands()[0].Flags().StringSlice("tags", nil, "Tags for the memory")
	cmd.Commands()[0].Flags().String("agent", "cli", "Agent name")
	cmd.Commands()[0].Flags().String("session", "", "Session ID")

	// Search flags
	cmd.Commands()[1].Flags().Int("limit", 10, "Max results")

	// List flags
	cmd.Commands()[2].Flags().String("agent", "", "Filter by agent")
	cmd.Commands()[2].Flags().String("tag", "", "Filter by tag")
	cmd.Commands()[2].Flags().Int("limit", 20, "Max results")

	// Export flags
	cmd.Commands()[5].Flags().StringP("output", "o", "", "Output file path")

	return cmd
}

func getMemoryStore(dir string) *memory.Store {
	if strings.HasPrefix(dir, "~/") {
		home, _ := os.UserHomeDir()
		dir = home + dir[1:]
	}
	return memory.NewStore(dir + "/store.json")
}
