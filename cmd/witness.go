package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/forge/sword/internal/pretty"
	"github.com/forge/sword/internal/witness"
	"github.com/spf13/cobra"
)

func witnessCmd() *cobra.Command {
	var dataDir string

	cmd := &cobra.Command{
		Use:   "witness",
		Short: "Cryptographic proof of agent actions",
		Long: `Record agent actions in a tamper-evident Merkle tree audit log.
Generate and verify cryptographic proofs for any action.

Every action. Every proof. No trust required.

Examples:
  forge witness record --agent=bot --type=file_write --target=main.go --session=s1
  forge witness prove s1 act-123
  forge witness verify <proof-json>
  forge witness root s1
  forge witness sessions
  forge witness log s1`,
	}

	cmd.PersistentFlags().StringVar(&dataDir, "dir", "", "Witness data directory (default: .forge/witness)")

	cmd.AddCommand(
		witnessRecordCmd(&dataDir),
		witnessProveCmd(&dataDir),
		witnessVerifyCmd(&dataDir),
		witnessRootCmd(&dataDir),
		witnessSessionsCmd(&dataDir),
		witnessLogCmd(&dataDir),
	)

	return cmd
}

func getWitness(dir *string) (*witness.Witness, error) {
	d := ""
	if dir != nil && *dir != "" {
		d = *dir
	} else {
		wd, _ := os.Getwd()
		d = wd + "/.forge/witness"
	}
	return witness.NewWitness(d)
}

func witnessRecordCmd(dir *string) *cobra.Command {
	var agentID, actionType, target, detail, sessionID string

	cmd := &cobra.Command{
		Use:   "record",
		Short: "Record an agent action",
		RunE: func(cmd *cobra.Command, args []string) error {
			w, err := getWitness(dir)
			if err != nil {
				return err
			}

			a := witness.Action{
				AgentID:   agentID,
				Type:      actionType,
				Target:    target,
				Detail:    detail,
				SessionID: sessionID,
				Timestamp: time.Now(),
			}

			hash, err := w.Record(a)
			if err != nil {
				return err
			}

			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Action recorded: %s", hash[:16]+"...")))
			return nil
		},
	}

	cmd.Flags().StringVar(&agentID, "agent", "", "Agent ID")
	cmd.Flags().StringVar(&actionType, "type", "message", "Action type")
	cmd.Flags().StringVar(&target, "target", "", "Target")
	cmd.Flags().StringVar(&detail, "detail", "", "Detail")
	cmd.Flags().StringVar(&sessionID, "session", "default", "Session ID")
	cmd.MarkFlagRequired("agent")

	return cmd
}

func witnessProveCmd(dir *string) *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "prove <session-id> <action-id>",
		Short: "Generate a proof for an action",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			w, err := getWitness(dir)
			if err != nil {
				return err
			}

			proof, err := w.Prove(args[0], args[1])
			if err != nil {
				return err
			}

			proof.Verified = w.Verify(proof)

			if asJSON {
				data, _ := json.MarshalIndent(proof, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Println(pretty.HeaderLine("Merkle Proof"))
			fmt.Printf("  Action:  %s\n", proof.ActionID)
			fmt.Printf("  Leaf:    %s...\n", proof.LeafHash[:16])
			fmt.Printf("  Root:    %s...\n", proof.RootHash[:16])
			fmt.Printf("  Depth:   %d\n", len(proof.ProofHashes))
			fmt.Printf("  Size:    %d leaves\n", proof.TreeSize)

			if proof.Verified {
				fmt.Println(pretty.SuccessLine("Proof verified"))
			} else {
				fmt.Println(pretty.Sprintf(pretty.RedF, "  ✗ Proof verification FAILED"))
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&asJSON, "json", false, "Output as JSON")
	return cmd
}

func witnessVerifyCmd(dir *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verify <proof-json>",
		Short: "Verify a proof",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var proof witness.Proof
			if err := json.Unmarshal([]byte(args[0]), &proof); err != nil {
				return fmt.Errorf("invalid proof JSON: %w", err)
			}

			if witness.VerifyStandalone(&proof) {
				fmt.Println(pretty.SuccessLine("Proof verified"))
				return nil
			}

			fmt.Println(pretty.Sprintf(pretty.RedF, "  ✗ Proof verification FAILED"))
			return fmt.Errorf("proof verification failed")
		},
	}
	return cmd
}

func witnessRootCmd(dir *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "root <session-id>",
		Short: "Get the Merkle root hash",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			w, err := getWitness(dir)
			if err != nil {
				return err
			}

			root, err := w.RootHash(args[0])
			if err != nil {
				return err
			}

			fmt.Println(root)
			return nil
		},
	}
	return cmd
}

func witnessSessionsCmd(dir *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sessions",
		Short: "List witnessed sessions",
		RunE: func(cmd *cobra.Command, args []string) error {
			w, err := getWitness(dir)
			if err != nil {
				return err
			}

			sessions := w.ListSessions()
			if len(sessions) == 0 {
				fmt.Println(pretty.InfoLine("No sessions"))
				return nil
			}

			fmt.Println(pretty.HeaderLine("Sessions"))
			for _, s := range sessions {
				actions := w.GetActions(s)
				fmt.Printf("  %-20s  %d actions\n", s, len(actions))
			}
			return nil
		},
	}
	return cmd
}

func witnessLogCmd(dir *string) *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "log <session-id>",
		Short: "Show action log for a session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			w, err := getWitness(dir)
			if err != nil {
				return err
			}

			actions := w.GetActions(args[0])
			if len(actions) == 0 {
				fmt.Println(pretty.InfoLine(fmt.Sprintf("No actions in session %s", args[0])))
				return nil
			}

			if asJSON {
				data, _ := json.MarshalIndent(actions, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Println(pretty.HeaderLine(fmt.Sprintf("Action Log: %s", args[0])))
			for _, a := range actions {
				fmt.Printf("  %s\n", witness.FormatAction(a))
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&asJSON, "json", false, "Output as JSON")
	return cmd
}
